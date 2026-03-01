package secrets

import (
	"fmt"
	"strings"
)

// OpenStore creates a secret store based on provider type.
// Supported providers:
// - "env": env vars / .env file (SECRET_* prefix), default
// - "bitwarden": read-only store backed by bw CLI
func OpenStore(provider, secretsDir string) (Store, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "env":
		envFile := secretsDir
		if envFile == "" {
			envFile = ".env"
		}
		return NewEnvStore(envFile, "SECRET_")
	case "bitwarden":
		return NewBitwardenStore()
	default:
		return nil, fmt.Errorf("unknown secrets provider: %q", provider)
	}
}
