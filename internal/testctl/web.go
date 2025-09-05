package testctl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// UI suites
func testWebMock(cfg *Config) error {
	info("==== Run Cypress (Mock) ====")
	// pick a port: prefer cfg.WebPort, but if busy choose a free one
	webPort := cfg.WebPort
	if busy, _ := isPortBusy(webPort); busy {
		p, err := chooseFreePort()
		if err != nil {
			return err
		}
		warn("[ports] %d busy; using free port %d for preview", webPort, p)
		webPort = p
	}
	defer func() { _ = killProcesses() }()
	// Build and preview with mocks
	if err := runEnvCmdStreaming(context.Background(), map[string]string{
		"VITE_USE_MOCKS": "1",
	}, "pnpm", "-C", "web", "build"); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	preview := exec.CommandContext(ctx, "pnpm", "-C", "web", "preview", "--port", fmt.Sprint(webPort))
	preview.Stdout = os.Stdout
	preview.Stderr = os.Stderr
	if err := preview.Start(); err != nil {
		return err
	}
	defer func() { _ = preview.Process.Kill() }()
	if err := waitHTTP(fmt.Sprintf("http://localhost:%d", webPort), 200, 60*time.Second); err != nil {
		return err
	}
	// Run cypress with dynamic baseUrl
	return runEnvCmdStreaming(context.Background(), map[string]string{
		"CYPRESS_BASE_URL": fmt.Sprintf("http://localhost:%d", webPort),
		// Signal to Cypress specs that we are running with mocks enabled
		"CYPRESS_USE_MOCKS": "1",
	}, "pnpm", "run", "test:e2e:run")
}
