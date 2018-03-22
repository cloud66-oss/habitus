package secrets

func GetProviders() map[string]SecretProvider {
	return map[string]SecretProvider{
		"file": &FileProvider{},
		"env":  &EnvProvider{},
	}
}
