package build

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dchest/uniuri"
	"github.com/docker/docker/builder/parser"
	"github.com/fsouza/go-dockerclient"
)

// Builder is a simple Dockerfile builder
type Builder struct {
	Build    *Manifest
	UniqueID string // unique id for this build sequence. This is used for automated builds

	config *tls.Config
	docker docker.Client
}

// NewBuilder creates a new builder in a new session
func NewBuilder(manifest *Manifest, uniqueID string) *Builder {
	b := Builder{}
	b.Build = manifest
	b.UniqueID = uniqueID

	certPath := os.Getenv("DOCKER_CERT_PATH")
	endpoint := os.Getenv("DOCKER_HOST")
	ca := path.Join(certPath, "ca.pem")
	cert := path.Join(certPath, "cert.pem")
	key := path.Join(certPath, "key.pem")
	client, err := docker.NewTLSClient(endpoint, cert, key, ca)
	b.docker = *client

	if err != nil {
		log.Fatalf("Failed to connect to Docker daemon %s", err.Error())
	}

	return &b
}

// StartBuild runs the build process end to end
func (b *Builder) StartBuild() error {
	for _, s := range b.Build.Steps {
		err := b.BuildStep(&s)
		if err != nil {
			return err
		}
	}

	// TODO: if this is a runtime step, push it up to the repo
	// TODO: Clear after yourself: images, containers, etc (optional for premium users)
	for _, s := range b.Build.Steps {
		if s.Keep {
			continue
		}

	}
	return nil
}

func (b *Builder) uniqueStepName(step *Step) string {
	if b.UniqueID == "" {
		return step.Name
	}

	return strings.ToLower(fmt.Sprintf("%s.%s", b.UniqueID, step.Name))
}

// BuildStep builds a single step
func (b *Builder) BuildStep(step *Step) error {
	// fix the Dockerfile
	err := b.replaceFromField(step)
	if err != nil {
		return err
	}

	// call Docker to build the Dockerfile (from the parsed file)
	// NOTE: This is not going to work when the build starts midflow as we don't know
	// NOTE: last step's session ID.
	// TODO: Fix this!
	// TODO: Make options configurable
	opts := docker.BuildImageOptions{
		Name:                b.uniqueStepName(step),
		Dockerfile:          filepath.Base(b.uniqueDockerfile(step)),
		NoCache:             true,
		SuppressOutput:      false,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		OutputStream:        os.Stdout, // TODO: use a multi writer to get a stream out for the API
		ContextDir:          b.Build.Workdir,
		/*
			AuthConfigs: docker.AuthConfigurations{
				Configs: map[string]docker.AuthConfiguration{
					"quay.io": {
						Username:      "foo",
						Password:      "bar",
						Email:         "baz",
						ServerAddress: "quay.io",
					},
				},
			},*/
	}

	err = b.docker.BuildImage(opts)
	if err != nil {
		return err
	}

	// if there are any artefacts to be picked up, create a container and copy them over
	if len(step.Artefacts) > 0 {
		// create a container
		container, err := b.createContainer(step)
		if err != nil {
			return err
		}

		for _, art := range step.Artefacts {
			err = b.copyToHost(&art, container.ID)
			if err != nil {
				return err
			}
		}

		// remove the created container
		removeOpts := docker.RemoveContainerOptions{
			ID:            container.ID,
			RemoveVolumes: true,
			Force:         true,
		}

		err = b.docker.RemoveContainer(removeOpts)
		if err != nil {
			return err
		}
	}

	// clean up the parsed docker file. It will remain there if there was a problem
	err = os.Remove(b.uniqueDockerfile(step))
	if err != nil {
		return err
	}

	return nil
}

// this replaces the FROM field in the Dockerfile to one with the previous step's unique name
// it stores the parsed result Dockefile in uniqueSessionName file
func (b *Builder) replaceFromField(step *Step) error {
	rwc, err := os.Open(path.Join(b.Build.Workdir, step.Dockerfile))
	if err != nil {
		return err
	}
	defer rwc.Close()

	node, err := parser.Parse(rwc)
	if err != nil {
		return err
	}

	for _, child := range node.Children {
		if child.Value == "from" {
			// found it. is it from anyone we know?
			if child.Next == nil {
				return errors.New("invalid Dockerfile. No valid FROM found")
			}

			imageName := child.Next.Value
			found, err := step.Manifest.FindStepByName(imageName)
			if err != nil {
				return err
			}

			if found != nil {
				child.Next.Value = b.uniqueStepName(found)
			}
		}
	}

	// did it have any effect?
	err = ioutil.WriteFile(b.uniqueDockerfile(step), []byte(dumpDockerfile(node)), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) copyToHost(a *Artefact, container string) error {
	// create the dest folder if not there
	err := os.MkdirAll(a.Dest, 0777)
	if err != nil {
		return err
	}

	dest, err := os.Create(path.Join(hostStorage(), a.Dest, filepath.Base(a.Source)))
	if err != nil {
		return err
	}
	defer dest.Close()

	opt := docker.CopyFromContainerOptions{
		OutputStream: dest,
		Container:    container,
		Resource:     a.Source,
	}

	return b.docker.CopyFromContainer(opt)
}

func (b *Builder) createContainer(step *Step) (*docker.Container, error) {
	config := docker.Config{
		AttachStdout: true,
		AttachStdin:  false,
		AttachStderr: false,
		Image:        b.uniqueStepName(step),
		Cmd:          []string{""},
	}
	opts := docker.CreateContainerOptions{
		Name:   b.uniqueStepName(step) + "." + uniuri.New(),
		Config: &config,
	}
	container, err := b.docker.CreateContainer(opts)
	if err != nil {
		return nil, err
	}

	return container, nil
}

func dumpDockerfile(node *parser.Node) string {
	str := ""
	str += node.Value

	if len(node.Flags) > 0 {
		str += fmt.Sprintf(" %q", node.Flags)
	}

	for _, n := range node.Children {
		str += dumpDockerfile(n) + "\n"
	}

	if node.Next != nil {
		for n := node.Next; n != nil; n = n.Next {
			if len(n.Children) > 0 {
				str += " " + dumpDockerfile(n)
			} else {
				str += " " + n.Value
			}
		}
	}

	return strings.TrimSpace(str)
}

func (b *Builder) uniqueDockerfile(step *Step) string {
	return filepath.Join(b.Build.Workdir, b.uniqueStepName(step))
}

func hostStorage() string {
	s := os.Getenv("CXBUILDER")
	if s == "" {
		return "/tmp"
	}

	return s
}
