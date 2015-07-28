package configuration

import (
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

// Artefact holds a parsed source for a build artefact
type Artefact struct {
	Order  int
	Step   Step
	Source string
	Dest   string
}

// Step Holds a single step in the build process
// Public structs. They are used to store the build for the builders
type Step struct {
	Order      int
	Name       string
	Dockerfile string
	ImageName  string
	Runtime    bool
	Artefacts  []Artefact
	Build      Build
}

// Build Holds the whole build process
type Build struct {
	Workdir string
	Steps   []Step
}

// Private structs. They are used to load from yaml
type step struct {
	Name       string
	Dockerfile string
	ImageName  string
	Runtime    bool
	Artefacts  []string
}

type build struct {
	Workdir string
	Steps   []step
}

// LoadBuildFromFile loads Build from a yaml file
func LoadBuildFromFile(file string) (*Build, error) {
	t := build{}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal([]byte(data), &t)
	if err != nil {
		return nil, err
	}

	return t.convertToBuild()
}

func (b *build) convertToBuild() (*Build, error) {
	r := Build{}
	r.Workdir = b.Workdir
	r.Steps = []Step{}

	for idx, s := range b.Steps {
		convertedStep := Step{}

		convertedStep.Build = r
		convertedStep.Dockerfile = s.Dockerfile
		convertedStep.ImageName = s.ImageName
		convertedStep.Name = s.Name
		convertedStep.Order = idx
		convertedStep.Runtime = s.Runtime
		convertedStep.Artefacts = []Artefact{}

		for kdx, a := range s.Artefacts {
			convertedArt := Artefact{}

			convertedArt.Order = kdx
			convertedArt.Step = convertedStep
			parts := strings.Split(a, ":")
			// TODO: Validate (both parts should exist)
			convertedArt.Source = parts[0]
			convertedArt.Dest = parts[1]

			convertedStep.Artefacts = append(convertedStep.Artefacts, convertedArt)
		}

		// TODO: validate (unique name,...)

		r.Steps = append(r.Steps, convertedStep)
	}

	return &r, nil
}
