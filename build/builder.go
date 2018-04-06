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
	"sync"

	"os/exec"

	"github.com/cloud66/habitus/configuration"
	"github.com/cloud66/habitus/squash"
	"github.com/dchest/uniuri"
	"github.com/dustin/go-humanize"
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
	wg        sync.WaitGroup
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
		if conf.UseTLS {
			certPath := b.Conf.DockerCert
			ca := path.Join(certPath, "ca.pem")
			cert := path.Join(certPath, "cert.pem")
			key := path.Join(certPath, "key.pem")
			client, err = docker.NewTLSClient(endpoint.String(), cert, key, ca)
		} else {
			client, err = docker.NewClient(endpoint.String())
		}
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

	var hostArtifactRoots []string
	if !b.Conf.KeepArtifacts {
		b.Conf.Logger.Debug("Collecting artifact information")
		hostArtifactRoots = b.collectHostArtifactRoots()
	}

	b.Conf.Logger.Debugf("Building %d steps", len(b.Build.Steps))
	for i, levels := range b.Build.buildLevels {
		for j, s := range levels {
			b.Conf.Logger.Debugf("Step %d - %s, image-name = '%s'", i+j+1, s.Label, b.uniqueStepName(&s))
		}
	}

	for i, levels := range b.Build.buildLevels {
		for j, s := range levels {
			b.wg.Add(1)
			go func(st Step) {
				b.Conf.Logger.Debugf("Step %d - Build for %s", i+j+1, st.Name)
				defer b.wg.Done()

				err := b.BuildStep(&st, i+j)
				if err != nil {
					b.Conf.Logger.Fatalf("Build for step %s failed due to %s", st.Name, err.Error())
				}
			}(s)
		}

		b.wg.Wait()
	}

	if !b.Conf.KeepArtifacts {
		// remove all artifacts created on the host
		for _, hostArtifactRoot := range hostArtifactRoots {
			b.Conf.Logger.Debugf("Removing artifact path: %s\n", hostArtifactRoot)
			// this path might be removed already due to overlapping
			// values; so we don't care if this fails
			os.RemoveAll(hostArtifactRoot)
		}
	}

	if b.Conf.KeepSteps {
		return nil
	}

	if len(b.Build.Steps) < 1 {
		b.Conf.Logger.Fatal("No build steps found")
	}

	// Clear after yourself: images, containers, etc (optional for premium users)
	// except last step
	for i, levels := range b.Build.buildLevels {
		for j, s := range levels {
			if (j == len(levels)-1) && (i != len(b.Build.buildLevels)-1) {
				b.Conf.Logger.Debugf("Step %d - Removing unwanted image %s", i+j+1, b.uniqueStepName(&s))
				rmiOptions := docker.RemoveImageOptions{Force: b.Conf.FroceRmImages, NoPrune: b.Conf.NoPruneRmImages}
				err := b.docker.RemoveImageExtended(b.uniqueStepName(&s), rmiOptions)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// collects all existing artifact roots that are created
// during the build process and saved on the host so they
// can be removed at the end of the build process
func (b *Builder) collectHostArtifactRoots() []string {
	var hostArtifactRoots []string
	if !b.Conf.KeepArtifacts {
		for _, step := range b.Build.Steps {
			for _, artifact := range step.Artifacts {
				// get the projected relative path to the host file
				absHostFile := path.Join(b.Conf.Workdir, artifact.Dest, filepath.Base(artifact.Source))
				// use a regex to hand path expansion (ie. ../../)
				relHostFile := regexp.MustCompile(fmt.Sprintf("^%s/+", b.Conf.Workdir)).ReplaceAllString(absHostFile, "")
				// remove trailing /
				relHostFile = regexp.MustCompile("/$").ReplaceAllString(relHostFile, "")
				parts := strings.Split(relHostFile, "/")
				currentPath := b.Conf.Workdir
				for _, part := range parts {
					currentPath = path.Join(currentPath, part)
					if _, err := os.Stat(currentPath); os.IsNotExist(err) {
						// everything from this point down should be deleted
						hostArtifactRoots = append(hostArtifactRoots, currentPath)
						break
					}
				}
			}
		}
	}
	return hostArtifactRoots
}

// provides a name for the image
// it always adds the UID (if provided) to the end of the name
// keeping the tag intact if it exists
func (b *Builder) uniqueStepName(step *Step) string {
	if b.UniqueID == "" {
		return step.Name
	}

	newName := step.Name
	if strings.Contains(step.Name, ":") {
		parts := strings.Split(step.Name, ":")
		newName = parts[0] + "-" + b.UniqueID + ":" + parts[1]
	} else {
		newName = step.Name + "-" + b.UniqueID
	}

	return strings.ToLower(newName)
}

// BuildStep builds a single step
func (b *Builder) BuildStep(step *Step, step_number int) error {
	b.Conf.Logger.Noticef("Step %d - Building %s from context '%s'", step_number+1, step.Name, b.Conf.Workdir)
	// fix the Dockerfile
	dockerfile, err := b.replaceFromField(step, step_number)
	if err != nil {
		return err
	}

	if step.Target != "" {
		dockerfile, err = readDockerfileToTarget(dockerfile, step.Target)
		if err != nil {
			return err
		}
	}

	if err := b.writeDockerfile(dockerfile, b.uniqueDockerfile(step), step_number); err != nil {
		return err
	}

	buildArgs := []docker.BuildArg{}
	for _, s := range b.Conf.BuildArgs {
		buildArgs = append(buildArgs, docker.BuildArg{Name: s.Key, Value: s.Value})
	}
	for k, v := range step.Args {
		buildArgs = append(buildArgs, docker.BuildArg{Name: k, Value: v})
	}
	// call Docker to build the Dockerfile (from the parsed file)

	b.Conf.Logger.Infof("Step %d - Building the %s image from %s", step_number+1, b.uniqueStepName(step), b.uniqueDockerfile(step))
	opts := docker.BuildImageOptions{
		Name:                b.uniqueStepName(step),
		NetworkMode:         b.Conf.Network,
		Dockerfile:          b.uniqueDockerfileName(step),
		NoCache:             b.Conf.NoCache || step.NoCache,
		SuppressOutput:      b.Conf.SuppressOutput,
		RmTmpContainer:      b.Conf.RmTmpContainers,
		ForceRmTmpContainer: b.Conf.ForceRmTmpContainer,
		OutputStream:        os.Stdout, // TODO: use a multi writer to get a stream out for the API
		ContextDir:          b.Conf.Workdir,
		BuildArgs:           buildArgs,
		CPUShares:           int64(b.Conf.DockerCPUShares),
	}

	if b.Conf.DockerCPUSetCPUs != "" {
		opts.CPUSetCPUs = b.Conf.DockerCPUSetCPUs
	}

	if b.Conf.DockerMemory != "" {
		// convery to int64
		memory, err := humanize.ParseBytes(b.Conf.DockerMemory)
		if err != nil {
			return err
		}
		opts.Memory = int64(memory)
	}

	if b.auth != nil {
		opts.AuthConfigs = *b.auth
	}

	err = b.docker.BuildImage(opts)

	if err != nil {
		return err
	}

	// if there are any artifacts to be picked up, create a container and copy them over
	// we also need a container if there are cleanup commands
	if len(step.Artifacts) > 0 || len(step.Cleanup.Commands) > 0 || step.Command != "" {
		b.Conf.Logger.Notice("Building container based on the image")

		// create a container
		container, err := b.createContainer(step)
		if err != nil {
			return err
		}

		if !b.Conf.NoSquash && len(step.Cleanup.Commands) > 0 {
			// start the container
			b.Conf.Logger.Noticef("Starting container %s to run cleanup commands", container.ID)
			startOpts := &docker.HostConfig{}
			err := b.docker.StartContainer(container.ID, startOpts)
			if err != nil {
				return err
			}

			for _, cmd := range step.Cleanup.Commands {
				b.Conf.Logger.Debugf("Running cleanup command %s on %s", cmd, container.ID)
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
						b.Conf.Logger.Errorf("Failed to run cleanup commands %s", err.Error())
					}
					success <- struct{}{}
				}()
				<-success
			}

			// commit the container
			cmtOpts := docker.CommitContainerOptions{
				Container: container.ID,
			}

			b.Conf.Logger.Debugf("Commiting the container %s", container.ID)
			img, err := b.docker.CommitContainer(cmtOpts)
			if err != nil {
				return err
			}

			b.Conf.Logger.Debugf("Stopping the container %s", container.ID)
			err = b.docker.StopContainer(container.ID, 0)
			if err != nil {
				return err
			}

			tmpFile, err := ioutil.TempFile("", "habitus-export-")
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

			b.Conf.Logger.Noticef("Exporting cleaned up container %s to %s", img.ID, tmpFile.Name())
			err = b.docker.ExportImage(expOpts)
			if err != nil {
				return err
			}

			// Squash
			sqTmpFile, err := ioutil.TempFile("", "habitus-export-")
			if err != nil {
				return err
			}
			defer sqTmpFile.Close()
			b.Conf.Logger.Noticef("Squashing image %s into %s", sqTmpFile.Name(), img.ID)

			squasher := squash.Squasher{Conf: b.Conf}
			err = squasher.Squash(tmpFile.Name(), sqTmpFile.Name(), b.uniqueStepName(step))
			if err != nil {
				return err
			}

			b.Conf.Logger.Debugf("Removing exported temp files")
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
			b.Conf.Logger.Debugf("Loading squashed image into docker")
			err = b.docker.LoadImage(loadOps)
			if err != nil {
				return err
			}

			err = os.Remove(sqTmpFile.Name())
			if err != nil {
				return err
			}
		}

		if len(step.Artifacts) > 0 {
			b.Conf.Logger.Noticef("Starting container %s to fetch artifact permissions", container.ID)
			startOpts := &docker.HostConfig{}
			err := b.docker.StartContainer(container.ID, startOpts)
			if err != nil {
				return err
			}

			permMap := make(map[string]int)
			if b.Conf.UseStatForPermissions {
				for _, art := range step.Artifacts {
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
						b.Conf.Logger.Errorf("Failed to fetch artifact permissions for %s: %s", art.Source, err.Error())
					}

					permsString := strings.Replace(strings.Replace(buf.String(), "'", "", -1), "\n", "", -1)
					perms, err := strconv.Atoi(permsString)
					if err != nil {
						b.Conf.Logger.Errorf("Failed to fetch artifact permissions for %s: %s", art.Source, err.Error())
					}
					permMap[art.Source] = perms
					b.Conf.Logger.Debugf("Permissions for %s is %d", art.Source, perms)
				}
			}

			b.Conf.Logger.Debugf("Stopping the container %s", container.ID)
			err = b.docker.StopContainer(container.ID, 0)
			if err != nil {
				return err
			}

			b.Conf.Logger.Noticef("Copying artifacts from %s", container.ID)

			for _, art := range step.Artifacts {
				if b.Conf.UseStatForPermissions {
					err = b.copyToHost(&art, container.ID, permMap)
				} else {
					err = b.copyToHost(&art, container.ID, nil)
				}
				if err != nil {
					return err
				}
			}
		}

		// any commands to run?
		if step.Command != "" {
			b.Conf.Logger.Noticef("Starting container %s to run commands", container.ID)
			startOpts := &docker.HostConfig{}

			err := b.docker.StartContainer(container.ID, startOpts)
			if err != nil {
				return err
			}

			execOpts := docker.CreateExecOptions{
				Container:    container.ID,
				AttachStdin:  false,
				AttachStdout: true,
				AttachStderr: true,
				Tty:          true,
				Cmd:          strings.Split(step.Command, " "),
			}
			execObj, err := b.docker.CreateExec(execOpts)
			if err != nil {
				return err
			}

			buf := new(bytes.Buffer)
			startExecOpts := docker.StartExecOptions{
				OutputStream: buf,
				ErrorStream:  os.Stderr,
				RawTerminal:  true,
				Detach:       false,
			}

			b.Conf.Logger.Noticef("Running command %s on container %s", execOpts.Cmd, container.ID)

			if err := b.docker.StartExec(execObj.ID, startExecOpts); err != nil {
				b.Conf.Logger.Errorf("Failed to execute command '%s' due to %s", step.Command, err.Error())
			}

			b.Conf.Logger.Noticef("\n%s", buf)

			inspect, err := b.docker.InspectExec(execObj.ID)
			if err != nil {
				return err
			}

			if inspect.ExitCode != 0 {
				b.Conf.Logger.Errorf("Running command %s on container %s exit with exit code %d", execOpts.Cmd, container.ID, inspect.ExitCode)
				return err
			} else {
				b.Conf.Logger.Noticef("Running command %s on container %s exit with exit code %d", execOpts.Cmd, container.ID, inspect.ExitCode)
			}

			b.Conf.Logger.Debugf("Stopping the container %s", container.ID)
			err = b.docker.StopContainer(container.ID, 0)
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

		b.Conf.Logger.Debugf("Removing built container %s", container.ID)
		err = b.docker.RemoveContainer(removeOpts)
		if err != nil {
			return err
		}
	}

	if step.AfterBuildCommand != "" && b.Conf.AllowAfterBuildCommands {
		b.Conf.Logger.Noticef("Step %d - Running command [%s] on host", step_number+1, step.AfterBuildCommand)
		stdoutStderr, err := exec.Command("sh", "-c", step.AfterBuildCommand).CombinedOutput()
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", stdoutStderr)
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
func (b *Builder) replaceFromField(step *Step, step_number int) (string, error) {
	b.Conf.Logger.Noticef("Step %d - Parsing and converting '%s'", step_number+1, step.Dockerfile)

	rwc, err := os.Open(path.Join(b.Conf.Workdir, step.Dockerfile))
	if err != nil {
		return "", err
	}
	defer rwc.Close()

	buffer, err := ioutil.ReadAll(rwc)
	if err != nil {
		return "", err
	}

	fromTag := regexp.MustCompile("FROM (.*)")
	if !fromTag.Match(buffer) {
		return "", errors.New("invalid Dockerfile. No valid FROM found")
	}

	imageNameAsBytes := fromTag.FindAllSubmatch(buffer, -1)
	imageName := string(imageNameAsBytes[0][1])
	found, err := step.Manifest.FindStepByName(imageName)
	if err != nil {
		return "", err
	}

	if found != nil {
		uniqueStepName := b.uniqueStepName(found)
		buffer = fromTag.ReplaceAll(buffer, []byte("FROM "+uniqueStepName))
	}

	return string(buffer), nil
}

func (b *Builder) writeDockerfile(dockerfile string, path string, stepNumber int) error {
	b.Conf.Logger.Debugf("Step %d - Writing the new Dockerfile into '%s'", stepNumber+1, path)
	err := ioutil.WriteFile(path, []byte(dockerfile), 0644)
	return err
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

func (b *Builder) copyToHost(a *Artifact, container string, perms map[string]int) error {
	// create the artifacts distination folder if not there
	destPath := path.Join(b.Conf.Workdir, a.Dest)
	err := os.MkdirAll(destPath, 0777)
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

	// create artifact file on the host
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

		destFile := path.Join(destPath, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(destFile, os.FileMode(hdr.Mode))
			os.Chown(destFile, hdr.Uid, hdr.Gid)
		case tar.TypeReg:
			b.Conf.Logger.Infof("Copying from %s to %s", a.Source, destFile)

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

		if b.Conf.UseStatForPermissions {
			b.Conf.Logger.Debugf("Setting file permissions for %s to %d", destFile, perms[a.Source])
			err = os.Chmod(destFile, os.FileMode(perms[a.Source])|0700)

			if err != nil {
				return err
			}
		}

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

func (b *Builder) uniqueDockerfileName(step *Step) string {
	if b.UniqueID != "" {
		return step.Dockerfile + "_" + b.UniqueID + ".generated"
	} else {
		return step.Dockerfile + ".generated"
	}
}

func (b *Builder) uniqueDockerfile(step *Step) string {
	return filepath.Join(b.Conf.Workdir, b.uniqueDockerfileName(step))
}
