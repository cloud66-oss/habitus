package build

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/cloud66/cxbuild/configuration"
	"github.com/cloud66/cxbuild/squash"
	"github.com/dchest/uniuri"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/fsouza/go-dockerclient"
)

// Builder is a simple Dockerfile builder
type Builder struct {
	Build    *Manifest
	UniqueID string // unique id for this build sequence. This is used for multi-tenanted environments
	Conf     *configuration.Config

	config *tls.Config
	docker docker.Client
	auth   *docker.AuthConfigurations
}

// NewBuilder creates a new builder in a new session
func NewBuilder(manifest *Manifest, conf *configuration.Config) *Builder {
	b := Builder{}
	b.Build = manifest
	b.UniqueID = conf.UniqueID
	b.Conf = conf

	certPath := b.Conf.DockerCert
	endpoint := b.Conf.DockerHost
	ca := path.Join(certPath, "ca.pem")
	cert := path.Join(certPath, "cert.pem")
	key := path.Join(certPath, "key.pem")
	client, err := docker.NewTLSClient(endpoint, cert, key, ca)
	if err != nil {
		b.Conf.Logger.Fatal(err.Error())
		return nil
	}

	b.docker = *client

	usr, err := user.Current()
	if err != nil {
		b.Conf.Logger.Fatalf("Failed to find the current user: %s", err.Error())
	}

	if _, err := os.Stat(filepath.Join(usr.HomeDir, ".dockercfg")); err == nil {
		authStream, err := os.Open(filepath.Join(usr.HomeDir, ".dockercfg"))
		if err != nil {
			b.Conf.Logger.Fatal("Unable to read .dockerconf file")
		}
		defer authStream.Close()

		auth, err := docker.NewAuthConfigurations(authStream)
		if err != nil {
			b.Conf.Logger.Fatalf("Invalid .dockerconf: %s", err.Error())
		}
		b.auth = auth
	}

	if err != nil {
		b.Conf.Logger.Fatalf("Failed to connect to Docker daemon %s", err.Error())
	}

	return &b
}

// StartBuild runs the build process end to end
func (b *Builder) StartBuild(startStep string) error {
	var steps []Step
	if startStep == "" {
		b.Conf.Logger.Notice("Starting the build chain")
		steps = b.Build.Steps
	} else {
		b.Conf.Logger.Notice("Starting the build chain from '%s'", startStep)
		for idx, s := range b.Build.Steps {
			if s.Name == startStep {
				steps = b.Build.Steps[idx:]
				break
			}
		}
	}

	for _, s := range steps {
		err := b.BuildStep(&s)
		if err != nil {
			return err
		}
	}

	// Clear after yourself: images, containers, etc (optional for premium users)
	for _, s := range steps {
		if s.Keep {
			continue
		}

		b.Conf.Logger.Debug("Removing unwanted image %s", b.uniqueStepName(&s))
		err := b.docker.RemoveImage(b.uniqueStepName(&s))
		if err != nil {
			return err
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
	b.Conf.Logger.Notice("Building %s", step.Name)
	// fix the Dockerfile
	err := b.replaceFromField(step)
	if err != nil {
		return err
	}

	// call Docker to build the Dockerfile (from the parsed file)
	opts := docker.BuildImageOptions{
		Name:                b.uniqueStepName(step),
		Dockerfile:          filepath.Base(b.uniqueDockerfile(step)),
		NoCache:             b.Conf.NoCache,
		SuppressOutput:      b.Conf.SuppressOutput,
		RmTmpContainer:      b.Conf.RmTmpContainers,
		ForceRmTmpContainer: b.Conf.ForceRmTmpContainer,
		OutputStream:        os.Stdout, // TODO: use a multi writer to get a stream out for the API
		ContextDir:          b.Build.Workdir,
	}

	if b.auth != nil {
		opts.AuthConfigs = *b.auth
	}

	err = b.docker.BuildImage(opts)
	if err != nil {
		return err
	}

	// if there are any artefacts to be picked up, create a container and copy them over
	// we also need a container if there are cleanup commands
	if len(step.Artefacts) > 0 || len(step.Cleanup.Commands) > 0 {
		b.Conf.Logger.Notice("Building container based on the image")

		// create a container
		container, err := b.createContainer(step)
		if err != nil {
			return err
		}

		if len(step.Cleanup.Commands) > 0 {
			// start the container
			b.Conf.Logger.Notice("Starting container %s to run cleanup commands", container.ID)
			startOpts := &docker.HostConfig{}
			err := b.docker.StartContainer(container.ID, startOpts)
			if err != nil {
				return err
			}

			for _, cmd := range step.Cleanup.Commands {
				b.Conf.Logger.Debug("Running cleanup command %s on %s", cmd, container.ID)
				// create an exec for the commands
				execOpts := docker.CreateExecOptions{
					Container:    container.ID,
					AttachStdin:  false,
					AttachStdout: true,
					AttachStderr: true,
					Tty:          false,
					Cmd:          strings.Split(cmd, " "),
				}
				execObj, err := b.docker.CreateExec(execOpts)
				if err != nil {
					return err
				}

				success := make(chan struct{})

				go func() {
					startExecOpts := docker.StartExecOptions{
						OutputStream: os.Stdout,
						ErrorStream:  os.Stderr,
						RawTerminal:  true,
					}

					if err := b.docker.StartExec(execObj.ID, startExecOpts); err != nil {
						b.Conf.Logger.Error("Failed to run cleanup commands %s", err.Error())
					}
					success <- struct{}{}
				}()
				<-success
			}

			// commit the container
			cmtOpts := docker.CommitContainerOptions{
				Container: container.ID,
			}

			b.Conf.Logger.Debug("Commiting the container %s", container.ID)
			img, err := b.docker.CommitContainer(cmtOpts)
			if err != nil {
				return err
			}

			b.Conf.Logger.Debug("Stopping the container %s", container.ID)
			err = b.docker.StopContainer(container.ID, 0)
			if err != nil {
				return err
			}

			tmpFile, err := ioutil.TempFile("", "cxbuild-export-")
			if err != nil {
				return err
			}
			defer tmpFile.Close()
			tarWriter, err := os.Create(tmpFile.Name())
			if err != nil {
				return err
			}
			defer tarWriter.Close()
			// save the container
			expOpts := docker.ExportImageOptions{
				Name:         img.ID,
				OutputStream: tarWriter,
			}

			b.Conf.Logger.Notice("Exporting cleaned up container %s to %s", img.ID, tmpFile.Name())
			err = b.docker.ExportImage(expOpts)
			if err != nil {
				return err
			}

			// Squash
			sqTmpFile, err := ioutil.TempFile("", "cxbuild-export-")
			if err != nil {
				return err
			}
			defer sqTmpFile.Close()
			b.Conf.Logger.Notice("Squashing image %s into %s", sqTmpFile.Name(), img.ID)

			squasher := squash.Squasher{Conf: b.Conf}
			err = squasher.Squash(tmpFile.Name(), sqTmpFile.Name(), b.uniqueStepName(step))
			if err != nil {
				return err
			}

			b.Conf.Logger.Debug("Removing exported temp files")
			err = os.Remove(tmpFile.Name())
			if err != nil {
				return err
			}
			// Load
			sqashedFile, err := os.Open(sqTmpFile.Name())
			if err != nil {
				return err
			}
			defer sqashedFile.Close()

			loadOps := docker.LoadImageOptions{
				InputStream: sqashedFile,
			}
			b.Conf.Logger.Debug("Loading squashed image into docker")
			err = b.docker.LoadImage(loadOps)
			if err != nil {
				return err
			}

			err = os.Remove(sqTmpFile.Name())
			if err != nil {
				return err
			}
		}

		if len(step.Artefacts) > 0 {
			b.Conf.Logger.Notice("Copying artefacts from %s", container.ID)

			for _, art := range step.Artefacts {
				err = b.copyToHost(&art, container.ID)
				if err != nil {
					return err
				}
			}
		}

		// remove the created container
		removeOpts := docker.RemoveContainerOptions{
			ID:            container.ID,
			RemoveVolumes: true,
			Force:         true,
		}

		b.Conf.Logger.Debug("Removing built container %s", container.ID)
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
	b.Conf.Logger.Notice("Parsing and converting '%s'", step.Dockerfile)

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

	destFile := path.Join(b.Build.Workdir, a.Dest, filepath.Base(a.Source))
	dest, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer dest.Close()

	opt := docker.CopyFromContainerOptions{
		OutputStream: dest,
		Container:    container,
		Resource:     a.Source,
	}

	b.Conf.Logger.Info("Copying from %s to %s", a.Source, destFile)

	return b.docker.CopyFromContainer(opt)
}

func (b *Builder) createContainer(step *Step) (*docker.Container, error) {
	config := docker.Config{
		AttachStdout: true,
		AttachStdin:  false,
		AttachStderr: true,
		Image:        b.uniqueStepName(step),
		Cmd:          []string{"/bin/bash"},
		Tty:          true,
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
