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

// installLlamaCUDA installs and builds llama.cpp with CUDA on Arch-like systems.
// It clones to ~/src/llama.cpp (or updates it), configures with CUDA enabled,
// preferring gcc-14/g++-14 as the CUDA host compiler when available, and builds
// libllama.so under build-cuda14.
func installLlamaCUDA() error {
    info("[llama] Installing prerequisites (Arch)…")
    // Base toolchain and optional BLAS (harmless even for CUDA builds)
    _ = runMaybeSudo("pacman", "-S", "--needed", "--noconfirm", "base-devel", "cmake", "git", "ninja", "openblas", "cblas", "lapack")
    // CUDA toolkit
    if err := runMaybeSudo("pacman", "-S", "--needed", "--noconfirm", "cuda"); err != nil {
        return fmt.Errorf("failed to install cuda via pacman: %w", err)
    }

    // Prefer GCC 14 for CUDA host compiler on Arch
    hostCXX := "/usr/bin/g++-14"
    if _, err := os.Stat(hostCXX); os.IsNotExist(err) {
        info("[llama] gcc-14 not found; attempting installation…")
        if err := runMaybeSudo("pacman", "-S", "--needed", "--noconfirm", "gcc14", "gcc14-libs"); err != nil {
            info("[llama] Could not install gcc14 automatically: %v", err)
            // Fallback to default g++ if gcc14 unavailable
            if p, lookErr := exec.LookPath("g++"); lookErr == nil {
                hostCXX = p
            } else {
                hostCXX = "/usr/bin/g++"
            }
        }
    }

    home, _ := os.UserHomeDir()
    srcDir := filepath.Join(home, "src")
    llamaDir := filepath.Join(srcDir, "llama.cpp")
    buildDir := filepath.Join(llamaDir, "build-cuda14")
    if _, err := os.Stat(llamaDir); os.IsNotExist(err) {
        if err := os.MkdirAll(srcDir, 0o755); err != nil {
            return err
        }
        info("[llama] Cloning llama.cpp into %s", llamaDir)
        if err := runCmdVerbose(context.Background(), "git", "clone", "https://github.com/ggerganov/llama.cpp.git", llamaDir); err != nil {
            return err
        }
    } else {
        info("[llama] Updating llama.cpp in %s", llamaDir)
        _ = runCmdVerbose(context.Background(), "git", "-C", llamaDir, "pull", "--ff-only")
    }

    info("[llama] Configuring CMake (CUDA) with host compiler: %s", hostCXX)
    if err := runCmdVerbose(context.Background(), "cmake",
        "-S", llamaDir,
        "-B", buildDir,
        "-DCMAKE_BUILD_TYPE=Release",
        "-DGGML_CUDA=ON",
        "-DCUDAToolkit_ROOT=/opt/cuda",
        "-DCMAKE_CUDA_HOST_COMPILER="+hostCXX,
    ); err != nil {
        return err
    }
    if err := runCmdVerbose(context.Background(), "cmake", "--build", buildDir, "-j"); err != nil {
        return err
    }

    soPath := filepath.Join(buildDir, "libllama.so")
    if fi, err := os.Stat(soPath); err != nil || fi.IsDir() {
        return fmt.Errorf("libllama.so not found at %s", soPath)
    }
    info("[llama] Built: %s", soPath)
    info("[llama] Next steps (fish shell, current session):")
    info("    set -gx LLAMA_CPP_DIR %s", llamaDir)
    info("    set -gx CGO_CFLAGS  \"-I$LLAMA_CPP_DIR -I$LLAMA_CPP_DIR/ggml/include\"")
    info("    set -gx CGO_LDFLAGS \"-L$LLAMA_CPP_DIR/build-cuda14 -lllama\"")
    info("    set -gx LD_LIBRARY_PATH \"$LLAMA_CPP_DIR/build-cuda14:/opt/cuda/lib64:$LD_LIBRARY_PATH\"")
    info("    set -gx CGO_ENABLED 1")
    return nil
}
