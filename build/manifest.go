package build

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
	Dest   string // this is only the folder. Filename comes from the source
}

// Step Holds a single step in the build process
// Public structs. They are used to store the build for the builders
type Step struct {
	Order      int
	Name       string
	Dockerfile string
	ImageName  string
	Keep       bool
	Artefacts  []Artefact
	Manifest   Manifest
}

// Manifest Holds the whole build process
type Manifest struct {
	Workdir string
	Steps   []Step
}

// Private structs. They are used to load from yaml
type step struct {
	Name       string
	Dockerfile string
	ImageName  string
	Keep       bool
	Artefacts  []string
}

type build struct {
	Workdir string
	Steps   []step
}

// LoadBuildFromFile loads Build from a yaml file
func LoadBuildFromFile(file string) (*Manifest, error) {
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

func (b *build) convertToBuild() (*Manifest, error) {
	r := Manifest{}
	r.Workdir = b.Workdir
	r.Steps = []Step{}

	for idx, s := range b.Steps {
		convertedStep := Step{}

		convertedStep.Manifest = r
		convertedStep.Dockerfile = s.Dockerfile
		convertedStep.ImageName = s.ImageName
		convertedStep.Name = s.Name
		convertedStep.Order = idx
		convertedStep.Keep = s.Keep
		convertedStep.Artefacts = []Artefact{}

		for kdx, a := range s.Artefacts {
			convertedArt := Artefact{}

			convertedArt.Order = kdx
			convertedArt.Step = convertedStep
			parts := strings.Split(a, ":")
			convertedArt.Source = parts[0]
			if len(parts) == 1 {
				// only one use the base
				convertedArt.Dest = "."
			} else {
				convertedArt.Dest = parts[1]
			}

			convertedStep.Artefacts = append(convertedStep.Artefacts, convertedArt)
		}

		// TODO: validate (unique name,...)

		r.Steps = append(r.Steps, convertedStep)
	}

	return &r, nil
}

// FindStepByName finds a step by name. Returns nil if not found
func (m *Manifest) FindStepByName(name string) (*Step, error) {
	for _, step := range m.Steps {
		if step.Name == name {
			return &step, nil
		}
	}

	return nil, nil
}
