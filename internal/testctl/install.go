package testctl

import (
	"context"
	"os"
	"path/filepath"
)

func installGo() error {
	info("Downloading Go modules...")
	return runCmdVerbose(context.Background(), "go", "mod", "download")
}

func installPy() error {
	info("Installing Python dependencies...")
	pyDir := filepath.Join("tests", "e2e_py")
	venv := filepath.Join(pyDir, ".venv")
	if _, err := os.Stat(venv); os.IsNotExist(err) {
		if err := os.MkdirAll(venv, 0o755); err != nil {
			return err
		}
	}
	// python3 -m venv
	if err := runCmdVerbose(context.Background(), "python3", "-m", "venv", venv); err != nil {
		return err
	}
	// pip install -r requirements.txt
	pip := filepath.Join(venv, "bin", "pip")
	req := filepath.Join(pyDir, "requirements.txt")
	if err := runCmdVerbose(context.Background(), pip, "install", "-r", req); err != nil {
		return err
	}
	info("Python dependencies installed (venv: %s).", venv)
	return nil
}
