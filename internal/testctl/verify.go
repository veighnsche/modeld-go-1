package testctl

import (
	"context"
	"fmt"
	"os/exec"
)

// verifyHostDocker checks that docker is installed and the service is enabled/active.
func verifyHostDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found in PATH: %w", err)
	}
	if err := runCmdVerbose(context.Background(), "docker", "--version"); err != nil {
		return fmt.Errorf("docker --version failed: %w", err)
	}
	// systemd checks (best-effort; some environments may vary)
	if err := runMaybeSudo("systemctl", "is-enabled", "docker"); err != nil {
		return fmt.Errorf("docker service is not enabled or systemctl check failed: %w", err)
	}
	if err := runMaybeSudo("systemctl", "is-active", "docker"); err != nil {
		return fmt.Errorf("docker service is not active or systemctl check failed: %w", err)
	}
	if err := runCmdVerbose(context.Background(), "getent", "group", "docker"); err != nil {
		return fmt.Errorf("docker group not found: %w", err)
	}
	return nil
}

// verifyHostAct checks that act is installed and accessible.
func verifyHostAct() error {
	if _, err := exec.LookPath("act"); err != nil {
		return fmt.Errorf("act not found in PATH: %w", err)
	}
	if err := runCmdVerbose(context.Background(), "act", "--version"); err != nil {
		return fmt.Errorf("act --version failed: %w", err)
	}
	return nil
}
