package testctl

import (
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
