package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// Onboard implements the "onboard" subcommand: write local YAML config/profile,
// choose secret provider, and prepare a runnable sandbox config.
func Onboard(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("onboard", flag.ExitOnError)
	workspaceRoot := fs.String("workspace-root", "", "Workspace root containing CommandGrid, GhostProxy, RootFS")
	profile := fs.String("profile", "dev", "Profile name to write under ~/.config/control-plane/profiles")
	secretsProvider := fs.String("secrets-provider", "", "Secret provider: env or bitwarden")
	nonInteractive := fs.Bool("non-interactive", false, "Disable prompts; fail if required values are missing")
	configPath := fs.String("config", "sandbox.yaml", "Project sandbox config output path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	root := *workspaceRoot
	if root == "" {
		root = detectWorkspaceRoot(cwd)
	}

	provider := strings.TrimSpace(*secretsProvider)
	if provider == "" && !*nonInteractive {
		provider = promptSecretsProvider()
	}
	if provider == "" {
		provider = "env"
	}

	if provider == "bitwarden" {
		if _, err := exec.LookPath("bw"); err != nil {
			return fmt.Errorf("bitwarden provider selected but bw CLI is not installed")
		}
		logger.Printf("bitwarden provider selected (expects BW_SESSION or interactive bw auth)")
	}

	cfg := defaultDevConfig(root)
	cfg.Defaults.SecretsProvider = provider

	if err := ensureConfigDirs(); err != nil {
		return fmt.Errorf("creating config dirs: %w", err)
	}

	cfgPath, err := commandGridConfigPath()
	if err != nil {
		return err
	}
	if err := writeYAMLFile(cfgPath, cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	profilePath, err := commandGridProfilePath(*profile)
	if err != nil {
		return err
	}
	if err := writeYAMLFile(profilePath, map[string]any{
		"name": *profile,
		"defaults": map[string]any{
			"secrets_provider": provider,
			"sandbox_config":   *configPath,
			"telemetry_profile": "local-disk",
		},
	}); err != nil {
		return fmt.Errorf("writing profile: %w", err)
	}

	// If local sandbox config doesn't exist, seed from example.
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		examplePath := filepath.Join(cfg.Paths.CommandGrid, "sandbox.yaml.example")
		if raw, readErr := os.ReadFile(examplePath); readErr == nil {
			if writeErr := os.WriteFile(*configPath, raw, 0644); writeErr != nil {
				return fmt.Errorf("writing %s from example: %w", *configPath, writeErr)
			}
			logger.Printf("wrote %s from %s", *configPath, examplePath)
		}
	}

	fmt.Printf("Onboard complete.\n- Config: %s\n- Profile: %s\n- Secrets provider: %s\n", cfgPath, profilePath, provider)
	return nil
}

func promptSecretsProvider() string {
	var choice string
	prompt := &survey.Select{
		Message: "Select secrets provider:",
		Options: []string{
			"env - Environment variables or .env file (SECRET_ANTHROPIC_KEY, etc.)",
			"bitwarden - Bitwarden vault via bw CLI (requires bw login, BW_SESSION)",
		},
		Default: "env - Environment variables or .env file (SECRET_ANTHROPIC_KEY, etc.)",
		Help:    "Use ↑/↓ arrows to move, Enter or Space to select",
	}
	if err := survey.AskOne(prompt, &choice); err != nil {
		return "env"
	}
	if strings.Contains(choice, "bitwarden") {
		return "bitwarden"
	}
	return "env"
}
