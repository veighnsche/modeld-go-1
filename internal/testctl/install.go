package testctl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Installers
func installJS() error {
	info("Installing JS dependencies...")
	// Try corepack to enable pnpm if not present
	if _, err := exec.LookPath("pnpm"); err != nil {
		if _, corepackErr := exec.LookPath("corepack"); corepackErr == nil {
			_ = runCmdVerbose(context.Background(), "corepack", "enable")
			_ = runCmdVerbose(context.Background(), "corepack", "prepare", "pnpm@9.7.1", "--activate")
		}
	}
	if _, err := exec.LookPath("pnpm"); err != nil {
		return fmt.Errorf("pnpm is required; install via 'npm i -g pnpm' or enable corepack")
	}
	if err := runCmdVerbose(context.Background(), "pnpm", "install", "--frozen-lockfile"); err != nil { return err }
	if err := runCmdVerbose(context.Background(), "pnpm", "-C", "web", "install", "--frozen-lockfile"); err != nil { return err }
	info("JS dependencies installed.")
	return nil
}

func installGo() error {
	info("Downloading Go modules...")
	return runCmdVerbose(context.Background(), "go", "mod", "download")
}

func installPy() error {
	info("Installing Python dependencies...")
	pyDir := filepath.Join("tests", "e2e_py")
	venv := filepath.Join(pyDir, ".venv")
	if _, err := os.Stat(venv); os.IsNotExist(err) {
		if err := os.MkdirAll(venv, 0o755); err != nil { return err }
	}
	// python3 -m venv
	if err := runCmdVerbose(context.Background(), "python3", "-m", "venv", venv); err != nil { return err }
	// pip install -r requirements.txt
	pip := filepath.Join(venv, "bin", "pip")
	req := filepath.Join(pyDir, "requirements.txt")
	if err := runCmdVerbose(context.Background(), pip, "install", "-r", req); err != nil { return err }
	info("Python dependencies installed (venv: %s).", venv)
	return nil
}
