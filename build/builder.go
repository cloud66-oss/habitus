package build

import (
	"fmt"
	"os/exec"

	"github.com/cloud66/cxbuild/configuration"
	"github.com/nu7hatch/gouuid"
)

// Builder is a simple Dockerfile builder
type Builder struct {
	Session string // unique session id for this build
}

// NewBuilder creates a new builder in a new session
func NewBuilder() *Builder {
	b := Builder{}
	u, _ := uuid.NewV4()
	b.Session = u.String()

	return &b
}

// BuildStep builds a single step
func (b *Builder) BuildStep(step *configuration.Step) {
	// call Docker to build the Dockerfile
	cmd := exec.Command("sh", "-c", fmt.Sprintf("cd '%s' && docker build -t '%s_%s' -f %s .", step.Build.Workdir, b.Session, step.ImageName, step.Dockerfile))
	cmd.Run()
	fmt.Print(cmd.Output())
	// TODO: if there are any artefacts to be picked up, create a container and copy them over
	// TODO: if this is a runtime step, push it up to the repo
}
