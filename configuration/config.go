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

type Server struct {
	ApiBinding                        string
	ApiPort                           int
	AuthenticatedSecretServerUser     string
	AuthenticatedSecretServerPassword string
	Logger                            logging.Logger
	UseAuthenticatedSecretServer      bool
}

// Config stores application configurations
type Config struct {
	Server
	Buildfile               string
	Workdir                 string
	NoCache                 bool
	SuppressOutput          bool
	RmTmpContainers         bool
	ForceRmTmpContainer     bool
	UniqueID                string
	Logger                  logging.Logger
	DockerHost              string
	DockerCert              string
	EnvVars                 TupleArray
	BuildArgs               TupleArray
	KeepSteps               bool
	KeepArtifacts           bool
	Network                 string
	NoSquash                bool
	NoPruneRmImages         bool
	UseTLS                  bool
	UseStatForPermissions   bool
	FroceRmImages           bool
	SecretService           bool
	AllowAfterBuildCommands bool
	SecretProviders         string
	DockerMemory            string
	DockerCPUSetCPUs        string
	DockerCPUShares         int
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
