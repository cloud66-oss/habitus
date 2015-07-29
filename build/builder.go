package build

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
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
	err := b.BuildContext()
	if err != nil {
		return err
	}

	for _, s := range b.Build.Steps {
		err = b.BuildStep(&s)
		if err != nil {
			return err
		}
	}

	return nil
}

// BuildStep builds a single step
func (b *Builder) BuildStep(step *Step) error {
	// call Docker to build the Dockerfile
	contextFile, err := os.OpenFile(b.contextFileName(), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer contextFile.Close()
	tarReader := io.Reader(contextFile)
	var buf bytes.Buffer

	opts := docker.BuildImageOptions{
		Name:                strings.ToLower(fmt.Sprintf("%s.%s", b.Session, step.Name)),
		Dockerfile:          step.Dockerfile,
		NoCache:             true,
		SuppressOutput:      true,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		InputStream:         tarReader,
		OutputStream:        &buf,
		//		ContextDir:          b.Build.Workdir, the new docker client can work with dirs so no need for taring?
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

	/*
		scanner := bufio.NewScanner(buf)
		for scanner.Scan() {
			fmt.Print(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	*/
	fmt.Print(buf.String())
	return nil
	// TODO: if there are any artefacts to be picked up, create a container and copy them over
	// TODO: if this is a runtime step, push it up to the repo
}

// BuildContext builds a tar file for the context
func (b *Builder) BuildContext() error {
	// REFACTOR: Ideally we should use native tarring
	cmd := exec.Command("tar", "-cf", b.contextFileName(), "-C", b.Build.Workdir, ".")
	return cmd.Run()
}

func (b *Builder) contextFileName() string {
	// TODO: for now we are putting them into /tmp
	return fmt.Sprintf("/tmp/%s.tar", b.Session)
}
