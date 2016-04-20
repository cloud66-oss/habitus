package build

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/cloud66/habitus/configuration"
	"github.com/cloud66/habitus/secrets"

	"gopkg.in/yaml.v2"
)

var (
	validTypes = []string{"file"}
)

// Artefact holds a parsed source for a build artefact
type Artefact struct {
	Step   Step
	Source string
	Dest   string // this is only the folder. Filename comes from the source
}

// Cleanup holds everything that's needed for a cleanup
type Cleanup struct {
	Commands []string
}

// holds a single secret
type Secret struct {
	Name  string
	Type  string
	Value string
}

// Step Holds a single step in the build process
// Public structs. They are used to store the build for the builders
type Step struct {
	Name       string
	Label      string
	Dockerfile string
	Artefacts  []Artefact
	Manifest   Manifest
	Cleanup    *Cleanup
	DependsOn  []*Step
	Command    string
	Secrets    []Secret
}

// Manifest Holds the whole build process
type Manifest struct {
	Steps           []Step
	IsPrivileged    bool
	SecretProviders map[string]secrets.SecretProvider

	buildLevels [][]Step
}

type cleanup struct {
	Commands []string `yaml:"commands"`
}

type secret struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

// Private structs. They are used to load from yaml
type step struct {
	Name       string            `yaml:"name"`
	Dockerfile string            `yaml:"dockerfile"`
	Artefacts  []string          `yaml:"artifacts"`
	Cleanup    *cleanup          `yaml:"cleanup"`
	DependsOn  []string          `yaml:"depends_on"`
	Command    string            `yaml:"command"`
	Secrets    map[string]secret `yaml:"secrets"`
}

// This is loaded from the build.yml file
type build struct {
	Version string          `yaml:"version"`
	Workdir string          `yaml:"work_dir"`
	Steps   map[string]step `yaml:"steps"`
}

// Habitus build namespace
type namespace struct {
	BuildConfig build `yaml:"build"`
	Config      *configuration.Config
}

// LoadBuildFromFile loads Build from a yaml file
func LoadBuildFromFile(config *configuration.Config) (*Manifest, error) {
	config.Logger.Noticef("Using '%s' as build file", config.Buildfile)

	n := namespace{Config: config}

	data, err := ioutil.ReadFile(config.Buildfile)
	if err != nil {
		return nil, err
	}

	data = parseForEnvVars(config, data)

	err = yaml.Unmarshal([]byte(data), &n)
	if err != nil {
		return nil, err
	}

	// check the version. for now we are going to support only one version
	// in future, version will select the parser
	if (n.BuildConfig.Version != "2016-02-13") && (n.BuildConfig.Version != "2016-03-14") {
		return nil, errors.New("Invalid build schema version")
	}

	return n.convertToBuild(n.BuildConfig.Version)
}

func (n *namespace) convertToBuild(version string) (*Manifest, error) {
	r := Manifest{
		SecretProviders: make(map[string]secrets.SecretProvider),
	}
	r.SecretProviders["file"] = &secrets.FileProvider{}

	r.IsPrivileged = false
	r.Steps = []Step{}

	for name, s := range n.BuildConfig.Steps {
		convertedStep := Step{}

		convertedStep.Manifest = r
		convertedStep.Dockerfile = s.Dockerfile
		convertedStep.Name = s.Name
		convertedStep.Label = name
		convertedStep.Artefacts = []Artefact{}
		convertedStep.Command = s.Command
		if s.Cleanup != nil && !n.Config.NoSquash {
			convertedStep.Cleanup = &Cleanup{Commands: s.Cleanup.Commands}
			r.IsPrivileged = true
		} else {
			convertedStep.Cleanup = &Cleanup{}
		}

		// TODO: should done through proper schema validation
		if version == "2016-03-14" {
			for name, s := range s.Secrets {
				convertedSecret := Secret{}
				convertedSecret.Name = name
				convertedSecret.Type = s.Type
				convertedSecret.Value = s.Value

				if !stringInSlice(s.Type, validTypes) {
					return nil, fmt.Errorf("Invalid type %s'", s.Type)
				}
				if !stringInSlice(s.Type, strings.Split(n.Config.SecretProviders, ",")) {
					return nil, fmt.Errorf("Unsupported type '%s'", s.Type)
				}

				r.SecretProviders[s.Type].RegisterSecret(name, s.Value)

				convertedStep.Secrets = append(convertedStep.Secrets, convertedSecret)
			}
		}

		for _, a := range s.Artefacts {
			convertedArt := Artefact{}

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

		// is it unique?
		for _, s := range r.Steps {
			if s.Name == convertedStep.Name {
				return nil, fmt.Errorf("Step name '%s' is not unique", convertedStep.Name)
			}
		}

		r.Steps = append(r.Steps, convertedStep)
	}

	// now that we have the Manifest built from the file, we can resolve dependencies
	for idx, step := range r.Steps {
		bStep := n.BuildConfig.Steps[step.Label]

		for _, d := range bStep.DependsOn {
			convertedStep, err := r.FindStepByLabel(d)
			if err != nil {
				return nil, err
			}
			if convertedStep == nil {
				return nil, fmt.Errorf("can't find step %s", d)
			}

			r.Steps[idx].DependsOn = append(r.Steps[idx].DependsOn, convertedStep)
		}
	}

	// build the dependency tree
	bl, err := r.serviceOrder(r.Steps)
	if err != nil {
		return nil, err
	}
	r.buildLevels = bl

	return &r, nil
}

func (m *Manifest) getStepsByLevel(level int) ([]Step, error) {
	if level >= len(m.buildLevels) {
		return nil, errors.New("level not available")
	}

	return m.buildLevels[level], nil
}

// takes in a list of steps and returns an array of steps ordered by their dependency order
// result[0] will be an array of all steps with no dependency
// result[1] will be an array of steps depending on one or more of result[0] steps and so on
func (m *Manifest) serviceOrder(mainList []Step) ([][]Step, error) {
	list := append([]Step(nil), mainList...)

	if len(list) == 0 {
		return [][]Step{}, nil
	}

	var result [][]Step

	// find all steps with no dependencies
	for {
		var level []Step
		for _, step := range list {
			if len(step.DependsOn) == 0 {
				level = append(level, step)
			}
		}

		// if none is found while there where items in the list, then we have a circular dependency somewhere
		if len(list) != 0 && len(level) == 0 {
			return nil, errors.New("Found circular dependency in services")
		}

		result = append(result, level)

		// now take out all of those found from the list of other items (they are now 'resolved')
		for idx, step := range list { // for every step
			stepDeps := append([]*Step(nil), step.DependsOn...) // clone the dependency list so we can remove items from it
			for kdx, dep := range stepDeps {                    // iterate through its dependeneis
				for _, resolved := range level { // and find any resolved step in them and take it out
					if resolved.Name == dep.Name {
						list[idx].DependsOn = append(list[idx].DependsOn[:kdx], list[idx].DependsOn[kdx+1:]...)
					}
				}
			}
		}

		// take out everything we have in this level from the list
		for _, s := range level {
			listCopy := append([]Step(nil), list...)
			for idx, l := range listCopy {
				if s.Name == l.Name {
					list = append(list[:idx], list[idx+1:]...)
				}
			}
		}

		// we are done
		if len(list) == 0 {
			break
		}
	}

	return result, nil
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

func (m *Manifest) FindStepByLabel(label string) (*Step, error) {
	for _, step := range m.Steps {
		if step.Label == label {
			return &step, nil
		}
	}

	return nil, nil
}

func parseForEnvVars(config *configuration.Config, value []byte) []byte {
	r, _ := regexp.Compile("_env\\((.*)\\)")

	matched := r.ReplaceAllFunc(value, func(s []byte) []byte {
		m := string(s)
		parts := r.FindStringSubmatch(m)

		if len(config.EnvVars) == 0 {
			return []byte(os.Getenv(parts[1]))
		} else {
			return []byte(config.EnvVars.Find(parts[1]))
		}
	})

	return matched
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
