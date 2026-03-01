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
	"strconv"
	"strings"
	"syscall"
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
	reuseProxy := fs.Bool("reuse-proxy", false, "Reuse an already-running GhostProxy instead of restarting it")
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

	ghostProxyBin := filepath.Join(devCfg.Paths.GhostProxy, "build", "ghostproxy")
	if artifacts, artErr := loadArtifactsFile(); artErr == nil && artifacts.Binaries.GhostProxy != "" {
		ghostProxyBin = artifacts.Binaries.GhostProxy
	}
	if err := ensureRunProxy(ghostProxyBin, proxyAddr, *detach, *reuseProxy); err != nil {
		return err
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

func ensureRunProxy(binPath, proxyAddr string, quiet bool, reuseProxy bool) error {
	healthy := proxyHealthy(proxyAddr)
	if healthy && reuseProxy {
		return nil
	}
	if healthy {
		if err := stopGhostProxyOnAddr(proxyAddr); err != nil {
			return err
		}
		for i := 0; i < 10; i++ {
			time.Sleep(150 * time.Millisecond)
			if !proxyHealthy(proxyAddr) {
				break
			}
		}
		if proxyHealthy(proxyAddr) {
			return fmt.Errorf("proxy at %s is still running after restart attempt (use --reuse-proxy to keep it)", proxyAddr)
		}
	}
	// Always mint and use a fresh admin token for dev run startup.
	os.Unsetenv("GHOSTPROXY_ADMIN_TOKEN")
	return startProxy(binPath, proxyAddr, quiet)
}

func stopGhostProxyOnAddr(proxyAddr string) error {
	port, err := proxyPort(proxyAddr)
	if err != nil {
		return err
	}
	out, err := exec.Command("lsof", "-t", "-i", "tcp:"+port, "-sTCP:LISTEN").Output()
	if err != nil {
		// If lsof found no listeners, treat as already stopped.
		if _, ok := err.(*exec.ExitError); ok {
			return nil
		}
		return fmt.Errorf("finding proxy pid on port %s: %w", port, err)
	}

	lines := strings.Fields(string(out))
	if len(lines) == 0 {
		return nil
	}

	killedAny := false
	for _, rawPID := range lines {
		pid, convErr := strconv.Atoi(strings.TrimSpace(rawPID))
		if convErr != nil || pid <= 0 {
			continue
		}
		cmdline, cmdErr := exec.Command("ps", "-p", rawPID, "-o", "command=").Output()
		if cmdErr != nil {
			continue
		}
		lc := strings.ToLower(string(cmdline))
		if !strings.Contains(lc, "ghostproxy") && !strings.Contains(lc, "llm-proxy") {
			continue
		}
		proc, findErr := os.FindProcess(pid)
		if findErr != nil {
			continue
		}
		_ = proc.Signal(syscall.SIGTERM)
		killedAny = true
	}
	if !killedAny {
		return fmt.Errorf("proxy port %s is already in use by a non-GhostProxy process; use --reuse-proxy to keep it", port)
	}
	return nil
}

func proxyPort(proxyAddr string) (string, error) {
	addr := strings.TrimSpace(proxyAddr)
	if addr == "" {
		return "", fmt.Errorf("proxy address is empty")
	}
	if strings.HasPrefix(addr, ":") {
		return strings.TrimPrefix(addr, ":"), nil
	}
	idx := strings.LastIndex(addr, ":")
	if idx == -1 || idx == len(addr)-1 {
		return "", fmt.Errorf("proxy address must include a port: %s", proxyAddr)
	}
	return addr[idx+1:], nil
}
