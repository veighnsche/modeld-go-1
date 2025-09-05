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

// testWebLiveHost runs the full UI suite against a live backend using host models.
func testWebLiveHost(cfg *Config) error {
	if !hasHostModels() {
		return errors.New("host models not found in $HOME/models/llm; cannot run live:host")
	}
	info("==== Run Cypress (Live:Host) ====")
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
	// Start server with host models
	modelsDir := filepath.Join(homeDir(), "models", "llm")
	defaultModel, err := firstGGUF(modelsDir)
	if err != nil {
		return fmt.Errorf("finding default model: %w", err)
	}
	srvCtx, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()
	srv := exec.CommandContext(srvCtx, "bash", "-lc", fmt.Sprintf("go run ./cmd/modeld --addr :%d --models-dir '%s' --default-model '%s' --cors-enabled --cors-origins '*'", apiPort, modelsDir, defaultModel))
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
	// Run cypress (headless) with dynamic baseUrl
	return runCypress(map[string]string{
		"CYPRESS_BASE_URL": fmt.Sprintf("http://localhost:%d", webPort),
	})
}
