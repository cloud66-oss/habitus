package build

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/nu7hatch/gouuid"
)

// Builder is a simple Dockerfile builder
type Builder struct {
	Build   *Manifest
	Session string // unique session id for this build

	config *tls.Config
	docker docker.Client
}

// NewBuilder creates a new builder in a new session
func NewBuilder(manifest *Manifest) *Builder {
	b := Builder{}
	b.Build = manifest
	u, _ := uuid.NewV4()
	b.Session = strings.Replace(u.String(), "-", "", -1)

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

	return nil
}

func (b *Builder) uniqueStepName(step *Step) string {
	return strings.ToLower(fmt.Sprintf("%s.%s", b.Session, step.Name))
}

// BuildStep builds a single step
func (b *Builder) BuildStep(step *Step) error {
	// call Docker to build the Dockerfile
	opts := docker.BuildImageOptions{
		Name:                b.uniqueStepName(step),
		Dockerfile:          step.Dockerfile,
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

	err := b.docker.BuildImage(opts)
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
	}
	// TODO: if this is a runtime step, push it up to the repo

	return nil
}

func (b *Builder) copyToHost(a *Artefact, container string) error {
	// create the dest folder if not there
	err := os.MkdirAll(a.Dest, 0777)
	if err != nil {
		return err
	}

	// TODO: make /tmp configurable
	dest, err := os.Create(path.Join("/tmp", a.Dest, filepath.Base(a.Source)))
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
		Name:   b.uniqueStepName(step),
		Config: &config,
	}
	container, err := b.docker.CreateContainer(opts)
	if err != nil {
		return nil, err
	}

	return container, nil
}
