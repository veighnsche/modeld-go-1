package testctl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	if err := runCmdVerbose(context.Background(), "pnpm", "install", "--frozen-lockfile"); err != nil {
		return err
	}
	if err := runCmdVerbose(context.Background(), "pnpm", "-C", "web", "install", "--frozen-lockfile"); err != nil {
		return err
	}
	info("JS dependencies installed.")
	return nil
}

// Host installers (Arch Linux focused)
func installHostDocker() error {
	info("[host] Installing Docker (Arch Linux)...")
	// Install docker with pacman
	if _, err := exec.LookPath("docker"); err != nil {
		if err := runMaybeSudo("pacman", "-S", "--needed", "--noconfirm", "docker"); err != nil {
			return fmt.Errorf("failed to install docker via pacman: %w", err)
		}
	} else {
		info("[host] docker already installed")
	}

	// Enable and start the docker service
	if err := runMaybeSudo("systemctl", "enable", "--now", "docker"); err != nil {
		return fmt.Errorf("failed to enable/start docker service: %w", err)
	}

	// Add current user to docker group
	usr := os.Getenv("USER")
	if usr == "" {
		usr = "$(whoami)"
	}
	if err := runMaybeSudo("usermod", "-aG", "docker", usr); err != nil {
		info("[host] Could not add user to docker group automatically: %v", err)
	} else {
		info("[host] Added user '%s' to docker group. You may need to re-login or run 'newgrp docker' for it to take effect.", usr)
	}

	// Smoke check
	_ = runCmdVerbose(context.Background(), "docker", "--version")
	return nil
}

func installHostAct() error {
	info("[host] Installing act...")
	if _, err := exec.LookPath("act"); err == nil {
		info("[host] act already installed")
		return nil
	}

	// Prefer AUR via yay if available (first), then paru
	if _, err := exec.LookPath("yay"); err == nil {
		if err := runCmdVerbose(context.Background(), "yay", "-S", "--noconfirm", "act-bin"); err != nil {
			info("[host] 'yay -S act-bin' failed, trying 'act'")
			if err2 := runCmdVerbose(context.Background(), "yay", "-S", "--noconfirm", "act"); err2 == nil {
				return nil
			}
		} else {
			return nil
		}
	} else if _, err := exec.LookPath("paru"); err == nil {
		if err := runCmdVerbose(context.Background(), "paru", "-S", "--noconfirm", "act-bin"); err != nil {
			info("[host] 'paru -S act-bin' failed, trying 'act'")
			if err2 := runCmdVerbose(context.Background(), "paru", "-S", "--noconfirm", "act"); err2 == nil {
				return nil
			}
		} else {
			return nil
		}
	}

	// Fallback: download binary tarball
	arch := runtime.GOARCH
	var tar string
	switch arch {
	case "amd64":
		tar = "act_Linux_x86_64.tar.gz"
	case "arm64":
		tar = "act_Linux_arm64.tar.gz"
	default:
		// Try x86_64 by default
		tar = "act_Linux_x86_64.tar.gz"
	}
	version := "v0.2.62" // fallback version; update as needed
	url := fmt.Sprintf("https://github.com/nektos/act/releases/download/%s/%s", version, tar)
	tmp := filepath.Join(os.TempDir(), tar)
	info("[host] Downloading %s", url)
	if err := runCmdVerbose(context.Background(), "curl", "-L", "-o", tmp, url); err != nil {
		return fmt.Errorf("failed to download act: %w", err)
	}
	info("[host] Extracting to /usr/local/bin (requires root)...")
	if err := runMaybeSudo("tar", "-C", "/usr/local/bin", "-xzf", tmp, "act"); err != nil {
		return fmt.Errorf("failed to extract act: %w", err)
	}
	_ = os.Remove(tmp)
	return runCmdVerbose(context.Background(), "act", "--version")
}

// Helpers
func runMaybeSudo(name string, args ...string) error {
	// If we can run without sudo (already root), do so; otherwise try sudo.
	if os.Geteuid() == 0 {
		return runCmdVerbose(context.Background(), name, args...)
	}
	if _, err := exec.LookPath("sudo"); err == nil {
		return runCmdVerbose(context.Background(), "sudo", append([]string{name}, args...)...)
	}
	// Try without sudo as a last resort
	return runCmdVerbose(context.Background(), name, args...)
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
