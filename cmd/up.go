package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	if err != nil && isContainerNameConflict(err) {
		logger.Printf("existing sandbox container name conflict for %q; attempting managed container eviction", *name)
		if evictErr := evictManagedSandboxByName(context.Background(), prov, *name); evictErr != nil {
			return fmt.Errorf("starting sandbox: %w (auto-evict failed: %v)", err, evictErr)
		}
		sandbox, err = orch.Up(context.Background(), *name)
	}
	if err != nil && shouldAutoRefreshProxyToken(cfg, err) {
		logger.Printf("proxy admin token missing; attempting GhostProxy restart with fresh token")
		if healErr := refreshProxyForUp(proxyAddr); healErr != nil {
			return fmt.Errorf("starting sandbox: %w (auto-refresh failed: %v)", err, healErr)
		}
		// Recreate orchestrator so it picks up the new token from env.
		orch = orchestrator.New(cfg, prov, store, proxyAddr, logger)
		sandbox, err = orch.Up(context.Background(), *name)
	}
	if err != nil {
		return fmt.Errorf("starting sandbox: %w", err)
	}

	fmt.Printf("Sandbox %s is running (id=%s)\n", sandbox.Name, sandbox.ID)
	return nil
}

func isContainerNameConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "container name") && strings.Contains(msg, "already in use")
}

func evictManagedSandboxByName(ctx context.Context, prov provisioner.Provisioner, name string) error {
	sandboxes, err := prov.List(ctx)
	if err != nil {
		return fmt.Errorf("listing managed sandboxes: %w", err)
	}
	for _, sb := range sandboxes {
		if sb != nil && sb.Name == name {
			return prov.Destroy(ctx, sb.ID)
		}
	}
	return fmt.Errorf("no managed sandbox named %q found (name may be owned by a non-managed container)", name)
}

func shouldAutoRefreshProxyToken(cfg *config.Config, err error) bool {
	if err == nil {
		return false
	}
	if !strings.Contains(err.Error(), "proxy-mode secrets require GHOSTPROXY_ADMIN_TOKEN") {
		return false
	}
	for _, secretCfg := range cfg.Secrets {
		if secretCfg.Mode == "proxy" {
			return true
		}
	}
	return false
}

func refreshProxyForUp(proxyAddr string) error {
	cwd, _ := os.Getwd()
	devCfg, err := loadOrDefaultDevConfig(cwd)
	if err != nil {
		return err
	}

	ghostProxyBin := filepath.Join(devCfg.Paths.GhostProxy, "build", "ghostproxy")
	if artifacts, artErr := loadArtifactsFile(); artErr == nil && artifacts.Binaries.GhostProxy != "" {
		ghostProxyBin = artifacts.Binaries.GhostProxy
	}

	// Keep behavior local-dev safe: only stop/restart known GhostProxy listeners.
	if proxyHealthy(proxyAddr) {
		if err := stopGhostProxyOnAddr(proxyAddr); err != nil {
			return err
		}
	}

	// Mint a new token and start a fresh proxy process.
	os.Unsetenv("GHOSTPROXY_ADMIN_TOKEN")
	return startProxy(ghostProxyBin, proxyAddr, true)
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
