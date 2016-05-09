package configuration

import (
	"errors"
	"strings"

	"github.com/op/go-logging"
)

type TupleItem struct {
	Key   string
	Value string
}

type TupleArray []TupleItem

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
	EnvVars             TupleArray
	BuildArgs           TupleArray
	KeepSteps           bool
	NoSquash            bool
	NoPruneRmImages     bool
	UseTLS              bool
	FroceRmImages       bool
	ApiPort             int
	ApiBinding          string
	SecretService       bool
	SecretProviders     string
}

func (i *TupleArray) String() string {
	return ""
}

func (i *TupleArray) Set(value string) error {
	parts := strings.Split(value, "=")

	if len(parts) != 2 {
		return errors.New("invalid key/value format (key=value)")
	}

	item := TupleItem{Key: parts[0], Value: parts[1]}
	*i = append(*i, item)
	return nil
}

func (i *TupleArray) Find(key string) string {
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
