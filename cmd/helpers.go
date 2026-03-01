package cmd

import (
	"os"

	"control-plane/pkg/secrets"
)

// userHomeDir returns the user's home directory.
func userHomeDir() (string, error) {
	return os.UserHomeDir()
}

func defaultSecretsDir() string {
	home, _ := userHomeDir()
	return home + "/.config/control-plane/secrets"
}

func openSecretStore(provider, secretsDir string) (secrets.Store, error) {
	sDir := secretsDir
	if sDir == "" {
		sDir = defaultSecretsDir()
	}
	return secrets.OpenStore(provider, sDir)
}
