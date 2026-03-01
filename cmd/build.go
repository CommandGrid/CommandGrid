package cmd

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type buildOptions struct {
	WorkspaceRoot string
	CommandGridDir string
	GhostProxyDir string
	RootFSDir string
	SkipImage bool
	SkipSelf bool
}

// Build implements the "build" subcommand: source-build required artifacts.
func Build(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	workspaceRoot := fs.String("workspace-root", "", "Workspace root containing CommandGrid, GhostProxy, RootFS")
	commandGridDir := fs.String("commandgrid-dir", "", "Path to CommandGrid repo (defaults to cwd)")
	ghostProxyDir := fs.String("ghostproxy-dir", "", "Path to GhostProxy repo")
	rootFSDir := fs.String("rootfs-dir", "", "Path to RootFS repo")
	skipImage := fs.Bool("skip-image", false, "Skip building RootFS image")
	skipSelf := fs.Bool("skip-self", false, "Skip building CommandGrid binary")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	cgDir := *commandGridDir
	if cgDir == "" {
		cgDir = cwd
	}
	root := *workspaceRoot
	if root == "" {
		root = detectWorkspaceRoot(cgDir)
	}
	gpDir := *ghostProxyDir
	if gpDir == "" {
		gpDir = filepath.Join(root, "GhostProxy")
	}
	rfDir := *rootFSDir
	if rfDir == "" {
		rfDir = filepath.Join(root, "RootFS")
	}

	opts := buildOptions{
		WorkspaceRoot: root,
		CommandGridDir: cgDir,
		GhostProxyDir: gpDir,
		RootFSDir: rfDir,
		SkipImage: *skipImage,
		SkipSelf: *skipSelf,
	}
	return runSourceBuild(opts, logger)
}

func runSourceBuild(opts buildOptions, logger *log.Logger) error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go is required but not found in PATH")
	}
	if !opts.SkipImage {
		if _, err := exec.LookPath("docker"); err != nil {
			return errors.New("docker is required for image build but not found in PATH")
		}
	}

	if err := validatePathExists(opts.CommandGridDir, "CommandGrid"); err != nil {
		return err
	}
	if err := validatePathExists(opts.GhostProxyDir, "GhostProxy"); err != nil {
		return err
	}
	if !opts.SkipImage {
		if err := validatePathExists(opts.RootFSDir, "RootFS"); err != nil {
			return err
		}
	}

	commandGridBin := filepath.Join(opts.CommandGridDir, "build", "control-plane")
	ghostProxyBin := filepath.Join(opts.GhostProxyDir, "build", "ghostproxy")
	rootfsImage := "rootfs:latest"

	if !opts.SkipSelf {
		logger.Printf("building CommandGrid in %s", opts.CommandGridDir)
		if err := runCmd(opts.CommandGridDir, "go", "build", "-o", commandGridBin, "."); err != nil {
			return fmt.Errorf("build CommandGrid: %w", err)
		}
	}

	logger.Printf("building GhostProxy in %s", opts.GhostProxyDir)
	if err := runCmd(opts.GhostProxyDir, "go", "build", "-o", ghostProxyBin, "."); err != nil {
		return fmt.Errorf("build GhostProxy: %w", err)
	}

	if !opts.SkipImage {
		logger.Printf("building RootFS image in %s", opts.RootFSDir)
		if err := runCmd(opts.RootFSDir, "docker", "build", "-t", rootfsImage, "."); err != nil {
			return fmt.Errorf("build RootFS image: %w", err)
		}
	}

	if err := ensureConfigDirs(); err != nil {
		return fmt.Errorf("ensure config dirs: %w", err)
	}
	artifactsPath, err := commandGridArtifactsPath()
	if err != nil {
		return err
	}
	if err := writeArtifacts(artifactsPath, commandGridBin, ghostProxyBin, rootfsImage); err != nil {
		return fmt.Errorf("write artifacts file: %w", err)
	}

	fmt.Printf("Build complete.\n- CommandGrid: %s\n- GhostProxy: %s\n- RootFS image: %s\n", commandGridBin, ghostProxyBin, rootfsImage)
	return nil
}

func runCmd(workingDir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func proxyHealthy(proxyAddr string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost" + proxyAddr + "/v1/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
