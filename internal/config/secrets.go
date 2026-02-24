package config

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const keyringService = "db-snap"

func SavePassword(profile, password string) error {
	if password == "" {
		return nil
	}
	return keyring.Set(keyringService, profile, password)
}

func LoadPassword(profile string) (string, error) {
	v, err := keyring.Get(keyringService, profile)
	if err != nil {
		return "", fmt.Errorf("load password from keyring for %s: %w", profile, err)
	}
	return v, nil
}

func DeletePassword(profile string) error {
	if err := keyring.Delete(keyringService, profile); err != nil {
		return err
	}
	return nil
}
