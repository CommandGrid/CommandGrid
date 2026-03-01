package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"control-plane/pkg/config"
	"control-plane/pkg/orchestrator"
	"control-plane/pkg/provisioner"
)

// Up implements the "up" subcommand: start a sandbox.
func Up(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	configPath := fs.String("config", "sandbox.yaml", "Path to sandbox.yaml")
	name := fs.String("name", "sandbox", "Sandbox name")
	secretsDir := fs.String("secrets-dir", "", "Path to .env file (env provider; default: .env in cwd)")
	secretsProvider := fs.String("secrets-provider", "env", "Secret provider: env or bitwarden")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Resolve relative host_path values in shared_dirs against the config
	// file's directory so Docker gets absolute bind mount paths.
	configDir, _ := filepath.Abs(filepath.Dir(*configPath))
	for i, sd := range cfg.SharedDirs {
		if !filepath.IsAbs(sd.HostPath) {
			cfg.SharedDirs[i].HostPath = filepath.Join(configDir, sd.HostPath)
		}
	}

	store, err := openSecretStore(*secretsProvider, *secretsDir)
	if err != nil {
		return fmt.Errorf("opening secret store: %w", err)
	}

	prov := resolveProvisioner(cfg)

	proxyAddr := cfg.Proxy.Addr
	if proxyAddr == "" {
		proxyAddr = ":8090"
	}

	orch := orchestrator.New(cfg, prov, store, proxyAddr, logger)

	sandbox, err := orch.Up(context.Background(), *name)
	if err != nil {
		return fmt.Errorf("starting sandbox: %w", err)
	}

	fmt.Printf("Sandbox %s is running (id=%s)\n", sandbox.Name, sandbox.ID)
	return nil
}

// resolveProvisioner creates the appropriate provisioner based on config.
func resolveProvisioner(cfg *config.Config) provisioner.Provisioner {
	switch cfg.SandboxMode {
	case "docker":
		return provisioner.NewDockerProvisioner("")
	case "unikraft":
		return provisioner.NewUnikraftProvisioner("")
	default:
		return provisioner.NewDockerProvisioner("")
	}
}
