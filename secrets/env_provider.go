package secrets

import (
	"os"
)

type EnvProvider struct {
	registry map[string]string
}

func (env_provider *EnvProvider) GetSecret(name string) (string, error) {
	key := env_provider.registry[name]
	dat := os.Getenv(key)
	return string(dat), nil
}

func (env_provider *EnvProvider) RegisterSecret(name string, value string) error {
	if env_provider.registry == nil {
		env_provider.registry = make(map[string]string)
	}
	env_provider.registry[name] = value
	return nil
}

