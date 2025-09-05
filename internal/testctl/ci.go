package testctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// listWorkflows returns the list of workflow files under .github/workflows/
func listWorkflows() ([]string, error) {
	dir := filepath.Join(".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			out = append(out, filepath.Join(dir, name))
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no workflow files found under .github/workflows/")
	}
	return out, nil
}

// runCIWorkflow runs a single workflow file with act. If useCatthehacker is true,
// we map ubuntu runners to the corresponding catthehacker images for better compatibility.
func runCIWorkflow(workflowFile string, useCatthehacker bool) error {
	if _, err := exec.LookPath("act"); err != nil {
		return fmt.Errorf("'act' is required. Install via 'testctl install host:act' or your package manager: %w", err)
	}
	args := []string{"-W", workflowFile}
	if useCatthehacker {
		// Map common ubuntu labels to catthehacker images
		// refs: https://github.com/catthehacker/docker_images
		args = append(args,
			"-P", "ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-22.04",
			"-P", "ubuntu-22.04=ghcr.io/catthehacker/ubuntu:act-22.04",
			"-P", "ubuntu-20.04=ghcr.io/catthehacker/ubuntu:act-20.04",
		)
	}
	info("[ci] Running workflow: %s", workflowFile)
	return runCmdStreaming(context.Background(), "act", args...)
}

// runCIAll runs all workflows discovered under .github/workflows.
func runCIAll(useCatthehacker bool) error {
	files, err := listWorkflows()
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := runCIWorkflow(f, useCatthehacker); err != nil {
			return err
		}
	}
	return nil
}
