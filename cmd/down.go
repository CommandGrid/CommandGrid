package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"

	"control-plane/pkg/config"
	"control-plane/pkg/orchestrator"
)

// Down implements the "down" subcommand: stop and destroy a sandbox.
func Down(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("down", flag.ExitOnError)
	configPath := fs.String("config", "sandbox.yaml", "Path to sandbox.yaml")
	id := fs.String("id", "", "Sandbox ID to stop")
	secretsDir := fs.String("secrets-dir", "", "Path to .env file (env provider)")
	secretsProvider := fs.String("secrets-provider", "env", "Secret provider: env or bitwarden")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *id == "" {
		return fmt.Errorf("--id is required")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
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

	if err := orch.Down(context.Background(), *id); err != nil {
		return fmt.Errorf("stopping sandbox: %w", err)
	}

	fmt.Printf("Sandbox %s destroyed\n", *id)
	return nil
}
