package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"

	"control-plane/pkg/config"
	"control-plane/pkg/orchestrator"
)

// Status implements the "status" subcommand: show sandbox status.
func Status(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	configPath := fs.String("config", "sandbox.yaml", "Path to sandbox.yaml")
	id := fs.String("id", "", "Sandbox ID (if empty, lists all)")
	secretsDir := fs.String("secrets-dir", "", "Path to .env file (env provider)")
	secretsProvider := fs.String("secrets-provider", "env", "Secret provider: env or bitwarden")
	if err := fs.Parse(args); err != nil {
		return err
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

	if *id != "" {
		// Show specific sandbox.
		sandbox, err := orch.Status(context.Background(), *id)
		if err != nil {
			return fmt.Errorf("getting status: %w", err)
		}
		fmt.Printf("%-12s %s\n", "ID:", sandbox.ID)
		fmt.Printf("%-12s %s\n", "Name:", sandbox.Name)
		fmt.Printf("%-12s %s\n", "Status:", sandbox.Status)
		if sandbox.IP != "" {
			fmt.Printf("%-12s %s\n", "IP:", sandbox.IP)
		}
	} else {
		// List all sandboxes.
		sandboxes, err := orch.List(context.Background())
		if err != nil {
			return fmt.Errorf("listing sandboxes: %w", err)
		}

		if len(sandboxes) == 0 {
			fmt.Println("No sandboxes found")
			return nil
		}

		fmt.Printf("%-16s %-20s %s\n", "ID", "NAME", "STATUS")
		for _, s := range sandboxes {
			short := s.ID
			if len(short) > 12 {
				short = short[:12]
			}
			fmt.Printf("%-16s %-20s %s\n", short, s.Name, s.Status)
		}
	}

	return nil
}
