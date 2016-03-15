package secrets

type SecretProvider interface {
	GetSecret(name string) (string, error)
	RegisterSecret(name string, value string) error
}
