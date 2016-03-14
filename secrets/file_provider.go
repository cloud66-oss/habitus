package secrets

import (
	"io/ioutil"
)

type FileProvider struct {
	registry map[string]string
}

func (f *FileProvider) GetSecret(name string) (string, error) {
	fl := f.registry[name]
	dat, err := ioutil.ReadFile(fl)
	if err != nil {
		return "", err
	}

	return string(dat), nil
}

func (f *FileProvider) RegisterSecret(name string, value string) error {
	if f.registry == nil {
		f.registry = make(map[string]string)
	}

	// TODO: check for duplicates and invalid names and values
	f.registry[name] = value

	return nil
}
