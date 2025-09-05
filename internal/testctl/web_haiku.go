package testctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// DO NOT MOCK THE HAIKU FOR TESTING!!!
// testWebHaikuHost starts a local API with host models, serves the web app without mocks,
// and runs only the Haiku Cypress spec against the live backend.
func testWebHaikuHost(cfg *Config) error {
	if !hasHostModels() {
		return errors.New("host models not found in $HOME/models/llm; cannot run haiku live test")
	}
	info("==== Run Cypress (Haiku Live:Host) ====")
	// determine API port (prefer 18080, else free)
	apiPort, err := preferOrFree(18080)
	if err != nil {
		return err
	}
	// determine web preview port (prefer cfg.WebPort, else free)
	webPort, err := preferOrFree(cfg.WebPort)
	if err != nil {
		return err
	}
	defer func() { _ = killProcesses() }()

	// Start server with host models and enable real inference
	modelsDir := filepath.Join(homeDir(), "models", "llm")
	defaultModel, err := firstGGUF(modelsDir)
	if err != nil {
		return fmt.Errorf("finding default model: %w", err)
	}
	srvCtx, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()
	llamaBin := findLlamaBin()
	srv := exec.CommandContext(srvCtx, "bash", "-lc", fmt.Sprintf(
		"go run ./cmd/modeld --addr :%d --models-dir '%s' --default-model '%s' --cors-enabled --cors-origins '*' --real-infer --llama-bin '%s'",
		apiPort, modelsDir, defaultModel, llamaBin,
	))
	srv.Stdout = os.Stdout
	srv.Stderr = os.Stderr
	if err := srv.Start(); err != nil {
		return err
	}
	// Track for unified cleanup
	TrackProcess(srv)
	defer func() { _ = srv.Process.Kill() }()
	if err := waitHTTP(fmt.Sprintf("http://localhost:%d/healthz", apiPort), 200, 60*time.Second); err != nil {
		return err
	}

	// Build and preview without mocks, pointing to API
	if err := buildWebWith(map[string]string{
		"VITE_USE_MOCKS":         "0",
		"VITE_API_BASE_URL":      fmt.Sprintf("http://localhost:%d", apiPort),
		"VITE_SEND_STREAM_FIELD": "true",
	}); err != nil {
		return err
	}
	pvCtx, pvCancel := context.WithCancel(context.Background())
	defer pvCancel()
	preview, err := startPreview(pvCtx, webPort)
	if err != nil {
		return err
	}
	defer func() { _ = preview.Process.Kill() }()
	if err := waitHTTP(fmt.Sprintf("http://localhost:%d", webPort), 200, 60*time.Second); err != nil {
		return err
	}

	// Run Cypress headless for only the haiku spec (centered), exporting API urls for optional checks
	return runCypress(map[string]string{
		"CYPRESS_BASE_URL":       fmt.Sprintf("http://localhost:%d", webPort),
		"CYPRESS_API_READY_URL":  fmt.Sprintf("http://localhost:%d/readyz", apiPort),
		"CYPRESS_API_STATUS_URL": fmt.Sprintf("http://localhost:%d/status", apiPort),
	}, "xvfb-run", "-a", "pnpm", "exec", "cypress", "run", "--config-file", "e2e/cypress.config.ts", "--spec", "e2e/specs/haiku_center.cy.ts")
}
