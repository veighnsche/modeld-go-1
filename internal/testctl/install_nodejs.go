package testctl

import (
    "context"
    "fmt"
    "os/exec"
)

// installNodeJS installs Node.js dependencies in the repo root and web/ using pnpm.
func installNodeJS() error {
    info("Installing Node.js dependencies...")
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
    info("Node.js dependencies installed.")
    return nil
}
