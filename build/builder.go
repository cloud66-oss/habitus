package build

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/nu7hatch/gouuid"
	"github.com/samalba/dockerclient"
)

// Builder is a simple Dockerfile builder
type Builder struct {
	Build   *Manifest
	Session string // unique session id for this build

	config *tls.Config
	docker dockerclient.Client
}

// NewBuilder creates a new builder in a new session
func NewBuilder(manifest *Manifest) *Builder {
	b := Builder{}
	b.Build = manifest
	u, _ := uuid.NewV4()
	b.Session = u.String()

	certPath := os.Getenv("DOCKER_CERT_PATH")
	caPool := x509.NewCertPool()
	severCert, err := ioutil.ReadFile(path.Join(certPath, "ca.pem"))
	if err != nil {
		log.Fatal("Could not load server certificate!")
	}
	caPool.AppendCertsFromPEM(severCert)

	cert, err := tls.LoadX509KeyPair(path.Join(certPath, "cert.pem"), path.Join(certPath, "key.pem"))
	if err != nil {
		log.Fatalf("Error reading certificates %s", err.Error())
	}
	b.config = &tls.Config{RootCAs: caPool, Certificates: []tls.Certificate{cert}}

	b.docker, err = dockerclient.NewDockerClient(os.Getenv("DOCKER_HOST"), b.config)
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

	image := dockerclient.BuildImage{
		DockerfileName: step.Dockerfile,
		Context:        tarReader,
	}
	rc, err := b.docker.BuildImage(&image)
	if err != nil {
		return err
	}
	defer rc.Close()

	stdoutBuffer := new(bytes.Buffer)
	stderrBuffer := new(bytes.Buffer)
	if _, err = stdcopy.StdCopy(stdoutBuffer, stderrBuffer, rc); err != nil {
		log.Fatal("cannot read logs from logs reader")
	}
	fmt.Print(strings.TrimSpace(stdoutBuffer.String()))
	fmt.Print(strings.TrimSpace(stderrBuffer.String()))

	// TODO: if there are any artefacts to be picked up, create a container and copy them over
	// TODO: if this is a runtime step, push it up to the repo

	return nil
}

// BuildContext builds a tar file for the context
func (b *Builder) BuildContext() error {
	// TODO: for now we are putting them into /tmp
	// REFACTOR: Ideally we should use native tarring
	cmd := exec.Command("tar", "-cf", b.contextFileName(), "-C", b.Build.Workdir, ".")
	return cmd.Run()
}

func (b *Builder) contextFileName() string {
	return fmt.Sprintf("/tmp/%s.tar", b.Session)
}
