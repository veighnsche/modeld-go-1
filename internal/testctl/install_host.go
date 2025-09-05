package testctl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Host installers (Arch and Debian/Ubuntu focused)
func installHostDocker() error {
	// If Docker already present, just ensure service and group.
	if _, err := exec.LookPath("docker"); err == nil {
		info("[host] docker already installed; ensuring service and group")
	} else {
		if isArchLike() {
			info("[host] Installing Docker (Arch Linux)...")
			if err := runMaybeSudo("pacman", "-S", "--needed", "--noconfirm", "docker"); err != nil {
				return fmt.Errorf("failed to install docker via pacman: %w", err)
			}
		} else if isDebianLike() {
			info("[host] Installing Docker (Debian/Ubuntu)...")
			// Simple path: install distro docker.io
			if err := runMaybeSudo("apt-get", "update"); err != nil {
				info("[host] apt-get update failed: %v", err)
			}
			if err := runMaybeSudo("apt-get", "install", "-y", "docker.io"); err != nil {
				return fmt.Errorf("failed to install docker via apt-get: %w", err)
			}
		} else {
			return fmt.Errorf("host:docker installer currently supports Arch and Debian/Ubuntu; on %s please install Docker manually", runtime.GOOS)
		}
	}

	// Enable and start the docker service (systemd)
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

	// Prefer Arch helpers if on Arch
	if isArchLike() {
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
	} else if isDebianLike() {
		// Try apt if available (may not exist on all releases)
		_ = runMaybeSudo("apt-get", "update")
		if err := runMaybeSudo("apt-get", "install", "-y", "act"); err == nil {
			return runCmdVerbose(context.Background(), "act", "--version")
		} else {
			info("[host] 'apt-get install act' failed, falling back to GitHub release: %v", err)
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
