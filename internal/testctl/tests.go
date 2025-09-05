package testctl

import (
	"context"
	"os"
	"path/filepath"
)

// Tests
func runGoTests() error {
	info("==== Run Go tests ====")
	return runCmdStreaming(context.Background(), "go", "test", "./...", "-v")
}

func runPyTests() error {
	info("==== Run Python E2E tests ====")
	pyDir := filepath.Join("tests", "e2e_py")
	venv := filepath.Join(pyDir, ".venv")
	pytest := filepath.Join(venv, "bin", "pytest")
	if _, err := os.Stat(pytest); os.IsNotExist(err) {
		info("Python venv not found; installing...")
		if err := installPy(); err != nil { return err }
	}
	return runCmdStreaming(context.Background(), pytest, "-q", pyDir)
}
