package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		p = "env"
	}

	sDir := strings.TrimSpace(secretsDir)
	// For env provider, default to a populated repo-root .env file.
	// This avoids confusing directory/file errors and gives actionable guidance.
	if p == "env" && sDir == "" {
		envPath, err := defaultEnvFilePath()
		if err != nil {
			return nil, err
		}
		sDir = envPath
	}
	return secrets.OpenStore(p, sDir)
}

func defaultEnvFilePath() (string, error) {
	root, err := findRepoRootFromCwd()
	if err != nil {
		// Fall back to cwd if repo root discovery fails.
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolving working directory for .env lookup: %w", err)
		}
	}

	envPath := filepath.Join(root, ".env")
	populated, err := isPopulatedEnvFile(envPath)
	if err != nil {
		return "", fmt.Errorf("checking .env file at %s: %w", envPath, err)
	}
	if !populated {
		return "", fmt.Errorf(
			"env secrets provider requires a populated .env file at %s; add required secrets (for example SECRET_ANTHROPIC_KEY=...) or use --secrets-provider bitwarden",
			envPath,
		)
	}

	return envPath, nil
}

func findRepoRootFromCwd() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		gitDir := filepath.Join(dir, ".git")
		if st, err := os.Stat(gitDir); err == nil && st.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not locate repository root from current directory")
}

func isPopulatedEnvFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[0]) == "" {
			continue
		}
		return true, nil
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}
