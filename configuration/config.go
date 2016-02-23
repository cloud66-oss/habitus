package configuration

import (
	"errors"
	"strings"

	"github.com/op/go-logging"
)

type EnvVarItem struct {
	Key   string
	Value string
}

type EnvVarsArray []EnvVarItem

// Config stores application configurations
type Config struct {
	Buildfile           string
	Workdir             string
	NoCache             bool
	SuppressOutput      bool
	RmTmpContainers     bool
	ForceRmTmpContainer bool
	UniqueID            string
	Logger              logging.Logger
	DockerHost          string
	DockerCert          string
	EnvVars             EnvVarsArray
	KeepSteps           bool
	NoSquash            bool
	NoPruneRmImages     bool
	FroceRmImages       bool
}

func (i *EnvVarsArray) String() string {
	return ""
}

func (i *EnvVarsArray) Set(value string) error {
	parts := strings.Split(value, "=")

	if len(parts) != 2 {
		return errors.New("invalid environment variable format (key=value)")
	}

	item := EnvVarItem{Key: parts[0], Value: parts[1]}
	*i = append(*i, item)
	return nil
}

func (i *EnvVarsArray) Find(key string) string {
	for _, item := range *i {
		if item.Key == key {
			return item.Value
		}
	}

	return ""
}

// CreateConfig creates a new configuration object
func CreateConfig() Config {
	return Config{}
}
