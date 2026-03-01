package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type DevConfig struct {
	Version       string         `yaml:"version"`
	WorkspaceRoot string         `yaml:"workspace_root"`
	Paths         DevConfigPaths `yaml:"paths"`
	Defaults      DevDefaults    `yaml:"defaults"`
}

type DevConfigPaths struct {
	CommandGrid string `yaml:"commandgrid"`
	GhostProxy  string `yaml:"ghostproxy"`
	RootFS      string `yaml:"rootfs"`
}

type DevDefaults struct {
	SandboxConfig   string `yaml:"sandbox_config"`
	SecretsProvider string `yaml:"secrets_provider"`
	TelemetryProfile string `yaml:"telemetry_profile"`
}

type BuildArtifacts struct {
	UpdatedAt string `yaml:"updated_at"`
	Binaries  struct {
		CommandGrid string `yaml:"commandgrid"`
		GhostProxy  string `yaml:"ghostproxy"`
	} `yaml:"binaries"`
	Images struct {
		RootFS string `yaml:"rootfs"`
	} `yaml:"images"`
}

func commandGridConfigDir() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "control-plane"), nil
}

func commandGridConfigPath() (string, error) {
	dir, err := commandGridConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func commandGridProfilePath(name string) (string, error) {
	dir, err := commandGridConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles", name+".yaml"), nil
}

func commandGridArtifactsPath() (string, error) {
	dir, err := commandGridConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "artifacts.yaml"), nil
}

func ensureConfigDirs() error {
	base, err := commandGridConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(base, "profiles"), 0700); err != nil {
		return err
	}
	return nil
}

func defaultDevConfig(workspaceRoot string) DevConfig {
	return DevConfig{
		Version:       "v1",
		WorkspaceRoot: workspaceRoot,
		Paths: DevConfigPaths{
			CommandGrid: filepath.Join(workspaceRoot, "CommandGrid"),
			GhostProxy:  filepath.Join(workspaceRoot, "GhostProxy"),
			RootFS:      filepath.Join(workspaceRoot, "RootFS"),
		},
		Defaults: DevDefaults{
			SandboxConfig:   "sandbox.yaml",
			SecretsProvider: "env",
			TelemetryProfile: "local-disk",
		},
	}
}

func writeYAMLFile(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func readDevConfig(path string) (DevConfig, error) {
	var cfg DevConfig
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func writeArtifacts(path, commandGridBin, ghostProxyBin, rootfsImage string) error {
	a := BuildArtifacts{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	a.Binaries.CommandGrid = commandGridBin
	a.Binaries.GhostProxy = ghostProxyBin
	a.Images.RootFS = rootfsImage
	return writeYAMLFile(path, a)
}

func detectWorkspaceRoot(commandGridDir string) string {
	// If running inside CommandGrid repo, use parent directory as workspace root.
	if filepath.Base(commandGridDir) == "CommandGrid" {
		return filepath.Dir(commandGridDir)
	}
	return commandGridDir
}

func validatePathExists(path, label string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%s not found at %s", label, path)
	}
	return nil
}
