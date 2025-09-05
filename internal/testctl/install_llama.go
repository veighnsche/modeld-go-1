package testctl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// installLlamaCUDA installs and builds llama.cpp with CUDA on Arch-like systems.
// It clones to ~/src/llama.cpp (or updates it), configures with CUDA enabled,
// preferring gcc-14/g++-14 as the CUDA host compiler when available, and builds
// libllama.so under build-cuda14.
func installLlamaCUDA() error {
	if !isArchLike() {
		return fmt.Errorf("llama:cuda installer currently supports Arch Linux-like distros; on %s please follow llama.cpp build docs for CUDA", runtime.GOOS)
	}
	info("[llama] Installing prerequisites (Arch)…")
	// Base toolchain and optional BLAS (harmless even for CUDA builds)
	_ = runCmdVerbose(context.Background(), "pacman", "-S", "--needed", "--noconfirm", "base-devel", "cmake", "git", "ninja", "openblas", "cblas", "lapack")
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
