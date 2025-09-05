package testctl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Host installers (Arch Linux focused)
func installHostDocker() error {
	if !isArchLike() {
		return fmt.Errorf("host:docker installer currently supports Arch Linux; on %s please install Docker manually", runtime.GOOS)
	}
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
	if !isArchLike() {
		return fmt.Errorf("host:act installer currently supports Arch Linux; on %s please install 'act' via your package manager or GitHub releases", runtime.GOOS)
	}
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

// runMaybeSudo tries sudo if not root, else runs directly.
func runMaybeSudo(name string, args ...string) error {
	if os.Geteuid() == 0 {
		return runCmdVerbose(context.Background(), name, args...)
	}
	if _, err := exec.LookPath("sudo"); err == nil {
		return runCmdVerbose(context.Background(), "sudo", append([]string{name}, args...)...)
	}
	return runCmdVerbose(context.Background(), name, args...)
}
