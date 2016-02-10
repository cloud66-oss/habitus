package build

import (
	"archive/tar"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/cloud66/habitus/configuration"
	"github.com/cloud66/habitus/squash"
	"github.com/dchest/uniuri"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/fsouza/go-dockerclient"
	"github.com/satori/go.uuid"
)

// Builder is a simple Dockerfile builder
type Builder struct {
	Build    *Manifest
	UniqueID string // unique id for this build sequence. This is used for multi-tenanted environments
	Conf     *configuration.Config

	config    *tls.Config
	docker    docker.Client
	auth      *docker.AuthConfigurations
	builderId string // unique id for this builder session (used internally)
}

// NewBuilder creates a new builder in a new session
func NewBuilder(manifest *Manifest, conf *configuration.Config) *Builder {
	b := Builder{}
	b.Build = manifest
	b.UniqueID = conf.UniqueID
	b.Conf = conf
	b.builderId = uuid.NewV4().String()

	endpoint, err := url.Parse(b.Conf.DockerHost)
	if err != nil {
		b.Conf.Logger.Fatalf("Invalid host: %s", err.Error())
		return nil
	}

	var client *docker.Client
	if endpoint.Scheme == "unix" {
		client, err = docker.NewClient(endpoint.String())
	} else {
		certPath := b.Conf.DockerCert
		ca := path.Join(certPath, "ca.pem")
		cert := path.Join(certPath, "cert.pem")
		key := path.Join(certPath, "key.pem")
		client, err = docker.NewTLSClient(endpoint.String(), cert, key, ca)
	}

	if err != nil {
		b.Conf.Logger.Fatal(err.Error())
		return nil
	}

	b.docker = *client

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		b.Conf.Logger.Fatalf("Failed to find the current home")
	}

	if _, err := os.Stat(filepath.Join(homeDir, ".dockercfg")); err == nil {
		authStream, err := os.Open(filepath.Join(homeDir, ".dockercfg"))
		if err != nil {
			b.Conf.Logger.Fatal("Unable to read .dockercfg file")
		}
		defer authStream.Close()

		auth, err := docker.NewAuthConfigurations(authStream)
		if err != nil {
			b.Conf.Logger.Fatalf("Invalid .dockercfg: %s", err.Error())
		}
		b.auth = auth
	}

	if err != nil {
		b.Conf.Logger.Fatalf("Failed to connect to Docker daemon %s", err.Error())
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

	if b.Conf.KeepSteps {
		return nil
	}

	// Clear after yourself: images, containers, etc (optional for premium users)
	// except last step
	for _, s := range b.Build.Steps[:len(b.Build.Steps)-1] {
		b.Conf.Logger.Debug("Removing unwanted image %s", b.uniqueStepName(&s))
		rmiOptions := docker.RemoveImageOptions{Force: b.Conf.FroceRmImages, NoPrune: b.Conf.NoPruneRmImages}
		err := b.docker.RemoveImageExtended(b.uniqueStepName(&s), rmiOptions)
		if err != nil {
			return err
		}
	}

	return nil
}

// provides a name for the image
// it always adds the UID (if provided) to the end of the name
// so it either be a tag or part of the provided tag
func (b *Builder) uniqueStepName(step *Step) string {
	if b.UniqueID == "" {
		return step.Name
	}

	newName := step.Name
	if strings.Contains(step.Name, ":") {
		newName = step.Name + "-" + b.UniqueID
	} else {
		newName = step.Name + ":" + b.UniqueID
	}

	return strings.ToLower(newName)
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
	b.Conf.Logger.Debug("Building the image from %s", filepath.Base(b.uniqueDockerfile(step)))
	opts := docker.BuildImageOptions{
		Name:                b.uniqueStepName(step),
		Dockerfile:          filepath.Base(b.uniqueDockerfile(step)),
		NoCache:             b.Conf.NoCache,
		SuppressOutput:      b.Conf.SuppressOutput,
		RmTmpContainer:      b.Conf.RmTmpContainers,
		ForceRmTmpContainer: b.Conf.ForceRmTmpContainer,
		OutputStream:        os.Stdout, // TODO: use a multi writer to get a stream out for the API
		ContextDir:          b.Conf.Workdir,
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

		if !b.Conf.NoSquash && len(step.Cleanup.Commands) > 0 {
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
			b.Conf.Logger.Notice("Starting container %s to fetch artefact permissions", container.ID)
			startOpts := &docker.HostConfig{}
			err := b.docker.StartContainer(container.ID, startOpts)
			if err != nil {
				return err
			}

			permMap := make(map[string]int)

			for _, art := range step.Artefacts {
				execOpts := docker.CreateExecOptions{
					Container:    container.ID,
					AttachStdin:  false,
					AttachStdout: true,
					AttachStderr: true,
					Tty:          false,
					Cmd:          []string{"stat", "--format='%a'", art.Source},
				}
				execObj, err := b.docker.CreateExec(execOpts)
				if err != nil {
					return err
				}

				buf := new(bytes.Buffer)
				startExecOpts := docker.StartExecOptions{
					OutputStream: buf,
					ErrorStream:  os.Stderr,
					RawTerminal:  false,
					Detach:       false,
				}

				if err := b.docker.StartExec(execObj.ID, startExecOpts); err != nil {
					b.Conf.Logger.Error("Failed to fetch artefact permissions for %s: %s", art.Source, err.Error())
				}

				permsString := strings.Replace(strings.Replace(buf.String(), "'", "", -1), "\n", "", -1)
				perms, err := strconv.Atoi(permsString)
				if err != nil {
					b.Conf.Logger.Error("Failed to fetch artefact permissions for %s: %s", art.Source, err.Error())
				}
				permMap[art.Source] = perms
				b.Conf.Logger.Debug("Permissions for %s is %d", art.Source, perms)
			}

			b.Conf.Logger.Debug("Stopping the container %s", container.ID)
			err = b.docker.StopContainer(container.ID, 0)
			if err != nil {
				return err
			}

			b.Conf.Logger.Notice("Copying artefacts from %s", container.ID)

			for _, art := range step.Artefacts {
				err = b.copyToHost(&art, container.ID, permMap)
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

	rwc, err := os.Open(path.Join(b.Conf.Workdir, step.Dockerfile))
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
	b.Conf.Logger.Debug("Writing the new Dockerfile into %s", step.Dockerfile+".generated")
	err = ioutil.WriteFile(b.uniqueDockerfile(step), []byte(dumpDockerfile(node)), 0644)
	if err != nil {
		return err
	}

	return nil
}

func overwrite(mpath string) (*os.File, error) {
	f, err := os.OpenFile(mpath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(mpath)
		if err != nil {
			return f, err
		}
	}
	return f, nil
}

func (b *Builder) copyToHost(a *Artefact, container string, perms map[string]int) error {
	// create the dest folder if not there
	err := os.MkdirAll(a.Dest, 0777)
	if err != nil {
		return err
	}

	var out bytes.Buffer

	opt := docker.DownloadFromContainerOptions{
		OutputStream: &out,
		Path:         a.Source,
	}

	err = b.docker.DownloadFromContainer(container, opt)
	if err != nil {
		return err
	}

	destFile := path.Join(b.Conf.Workdir, a.Dest, filepath.Base(a.Source))

	tr := tar.NewReader(&out)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeReg:
			b.Conf.Logger.Info("Copying from %s to %s", a.Source, destFile)

			dest, err := os.Create(destFile)
			if err != nil {
				return err
			}
			defer dest.Close()

			if _, err := io.Copy(dest, tr); err != nil {
				return err
			}
		default:
			return errors.New("Invalid header type")
		}
	}

	b.Conf.Logger.Debug("Setting file permissions for %s to %d", destFile, perms[a.Source])
	err = os.Chmod(destFile, os.FileMode(perms[a.Source])|0700)
	if err != nil {
		return err
	}

	return nil
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

	r, _ := regexp.Compile("/?[^a-zA-Z0-9_-]+")
	containerName := r.ReplaceAllString(b.uniqueStepName(step), "-") + "." + uniuri.New()
	opts := docker.CreateContainerOptions{
		Name:   containerName,
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
	return filepath.Join(b.Conf.Workdir, step.Dockerfile) + ".generated"
}
