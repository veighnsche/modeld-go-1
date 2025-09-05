package testctl

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
)

// findLlamaBin attempts to locate a llama.cpp server binary for local dev.
// Preference order:
// 1) $HOME/apps/llama.cpp/build/bin/llama-server
// 2) Jan-managed path under ~/.local/share/Jan/...
// 3) PATH lookup via exec.LookPath("llama-server")
func findLlamaBin() string {
    candidates := []string{
        filepath.Clean(filepath.Join(homeDir(), "apps", "llama.cpp", "build", "bin", "llama-server")),
        filepath.Clean(filepath.Join(homeDir(), ".local", "share", "Jan", "data", "llamacpp", "backends", "b6293", "linux-avx2-cuda-cu12.0-x64", "build", "bin", "llama-server")),
    }
    for _, p := range candidates {
        if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
            return p
        }
    }
    if lp, err := exec.LookPath("llama-server"); err == nil {
        return lp
    }
    // Fallback to a common name; modeld will error if it truly does not exist
    return "llama-server"
}

// buildWebWith builds the web app with the provided environment variables.
func buildWebWith(env map[string]string) error {
    return runEnvCmdStreaming(context.Background(), env, "pnpm", "-C", "web", "build")
}

// startPreview starts the Vite preview on the given port and returns the running *exec.Cmd.
func startPreview(ctx context.Context, port int) (*exec.Cmd, error) {
    cmd := exec.CommandContext(ctx, "pnpm", "-C", "web", "preview", "--port", fmt.Sprint(port))
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    // Track for unified cleanup
    TrackProcess(cmd)
    return cmd, nil
}

// runCypress runs Cypress with the provided environment. If no args are given,
// it runs the default script: `pnpm run test:e2e:run`.
func runCypress(env map[string]string, args ...string) error {
    if len(args) == 0 {
        return runEnvCmdStreaming(context.Background(), env, "pnpm", "run", "test:e2e:run")
    }
    // Allow fully custom invocation (e.g., xvfb-run pnpm exec cypress run ...)
    return runEnvCmdStreaming(context.Background(), env, args[0], args[1:]...)
}
