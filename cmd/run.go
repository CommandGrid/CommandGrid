package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"control-plane/pkg/config"
	"gopkg.in/yaml.v3"
)

// Run implements "run": preflight, optional source build, proxy bootstrap, then sandbox up.
func Run(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "sandbox.yaml", "Path to sandbox.yaml")
	name := fs.String("name", "sandbox", "Sandbox name")
	secretsProvider := fs.String("secrets-provider", "", "Secret provider: env or bitwarden")
	secretsDir := fs.String("secrets-dir", "", "Path to .env file (for env provider)")
	autoBuild := fs.Bool("auto-build", true, "Build required source artifacts before run")
	detach := fs.Bool("detach", false, "Do not print proxy bootstrap details")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	devCfg, err := loadOrDefaultDevConfig(cwd)
	if err != nil {
		return err
	}

	provider := *secretsProvider
	if provider == "" {
		provider = devCfg.Defaults.SecretsProvider
		if provider == "" {
			provider = "env"
		}
	}

	if *autoBuild {
		if err := runSourceBuild(buildOptions{
			WorkspaceRoot:  devCfg.WorkspaceRoot,
			CommandGridDir: devCfg.Paths.CommandGrid,
			GhostProxyDir:  devCfg.Paths.GhostProxy,
			RootFSDir:      devCfg.Paths.RootFS,
			SkipSelf:       true,
		}, logger); err != nil {
			return err
		}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	proxyAddr := cfg.Proxy.Addr
	if proxyAddr == "" {
		proxyAddr = ":8090"
	}

	if !proxyHealthy(proxyAddr) {
		ghostProxyBin := filepath.Join(devCfg.Paths.GhostProxy, "build", "ghostproxy")
		if artifacts, artErr := loadArtifactsFile(); artErr == nil && artifacts.Binaries.GhostProxy != "" {
			ghostProxyBin = artifacts.Binaries.GhostProxy
		}
		if err := startProxy(ghostProxyBin, proxyAddr, *detach); err != nil {
			return err
		}
	}

	upArgs := []string{
		"--config", *configPath,
		"--name", *name,
		"--secrets-provider", provider,
	}
	if *secretsDir != "" {
		upArgs = append(upArgs, "--secrets-dir", *secretsDir)
	}
	return Up(upArgs, logger)
}

func loadOrDefaultDevConfig(commandGridDir string) (DevConfig, error) {
	cfgPath, err := commandGridConfigPath()
	if err != nil {
		return DevConfig{}, err
	}
	cfg, err := readDevConfig(cfgPath)
	if err == nil {
		return cfg, nil
	}
	root := detectWorkspaceRoot(commandGridDir)
	return defaultDevConfig(root), nil
}

func loadArtifactsFile() (BuildArtifacts, error) {
	path, err := commandGridArtifactsPath()
	if err != nil {
		return BuildArtifacts{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return BuildArtifacts{}, err
	}
	var artifacts BuildArtifacts
	if err := yaml.Unmarshal(raw, &artifacts); err != nil {
		return BuildArtifacts{}, err
	}
	return artifacts, nil
}

func startProxy(binPath, proxyAddr string, quiet bool) error {
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("ghostproxy binary not found at %s (run `control-plane build`)", binPath)
	}
	env := os.Environ()
	if os.Getenv("GHOSTPROXY_ADMIN_TOKEN") == "" {
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			return fmt.Errorf("generating admin token: %w", err)
		}
		adminToken := "session-" + hex.EncodeToString(token)
		env = append(env, "GHOSTPROXY_ADMIN_TOKEN="+adminToken)
		os.Setenv("GHOSTPROXY_ADMIN_TOKEN", adminToken)
	}
	cmd := exec.Command(binPath, "-addr", proxyAddr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting ghostproxy: %w", err)
	}
	if !quiet {
		fmt.Printf("Started GhostProxy pid=%d on %s\n", cmd.Process.Pid, proxyAddr)
	}

	for i := 0; i < 10; i++ {
		time.Sleep(300 * time.Millisecond)
		if proxyHealthy(proxyAddr) {
			return nil
		}
	}
	return fmt.Errorf("ghostproxy did not become healthy on %s", proxyAddr)
}
