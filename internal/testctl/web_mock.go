package testctl

import (
	"context"
	"fmt"
	"time"
)

// UI suites
func testWebMock(cfg *Config) error {
	info("==== Run Cypress (Mock) ====")
	// pick a port: prefer cfg.WebPort, but if busy choose a free one
	webPort, err := preferOrFree(cfg.WebPort)
	if err != nil {
		return err
	}
	defer func() { _ = killProcesses() }()
	// Build and preview with mocks
	if err := buildWebWith(map[string]string{
		"VITE_USE_MOCKS": "1",
	}); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	preview, err := startPreview(ctx, webPort)
	if err != nil {
		return err
	}
	defer func() { _ = preview.Process.Kill() }()
	if err := waitHTTP(fmt.Sprintf("http://localhost:%d", webPort), 200, 60*time.Second); err != nil {
		return err
	}
	// Run cypress with dynamic baseUrl
	return runCypress(map[string]string{
		"CYPRESS_BASE_URL": fmt.Sprintf("http://localhost:%d", webPort),
		// Signal to Cypress specs that we are running with mocks enabled
		"CYPRESS_USE_MOCKS": "1",
	})
}
