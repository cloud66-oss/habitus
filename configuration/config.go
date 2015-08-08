package configuration

import "github.com/op/go-logging"

// Config stores application configurations
type Config struct {
	Buildfile           string
	StartStep           string
	NoCache             bool
	SuppressOutput      bool
	RmTmpContainers     bool
	ForceRmTmpContainer bool
	UniqueID            string
	Logger              logging.Logger
	DockerHost          string
	DockerCert          string
}

// CreateConfig creates a new configuration object
func CreateConfig() Config {
	return Config{}
}
