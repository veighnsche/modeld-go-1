package e2e

import (
    "bytes"
    "context"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"

    "modeld/internal/httpapi"
    "modeld/internal/manager"
    "modeld/internal/registry"
)

// createTempModelsDir creates a temporary directory populated with empty .gguf files
// and returns the directory path and the list of model IDs (filenames).
func createTempModelsDir(t *testing.T, names ...string) (string, []string) {
    t.Helper()
    dir := t.TempDir()
    for _, n := range names {
        p := filepath.Join(dir, n)
        if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
            t.Fatalf("write temp model %s: %v", p, err)
        }
    }
    return dir, names
}

func newServerForDir(t *testing.T, modelsDir string, budgetMB, marginMB int, defaultModel string) (*httptest.Server, *manager.Manager) {
    t.Helper()
    reg, err := registry.NewGGUFScanner().Scan(modelsDir)
    if err != nil {
        t.Fatalf("scan models: %v", err)
    }
    mgr := manager.New(reg, budgetMB, marginMB, defaultModel)
    mux := httpapi.NewMux(mgr)
    srv := httptest.NewServer(mux)
    t.Cleanup(srv.Close)
    return srv, mgr
}

// newServerForDirWithConfig allows configuring queue/backpressure behavior for tests.
func newServerForDirWithConfig(t *testing.T, modelsDir string, cfg manager.ManagerConfig) (*httptest.Server, *manager.Manager) {
    t.Helper()
    reg, err := registry.NewGGUFScanner().Scan(modelsDir)
    if err != nil {
        t.Fatalf("scan models: %v", err)
    }
    cfg.Registry = reg
    mgr := manager.NewWithConfig(cfg)
    mux := httpapi.NewMux(mgr)
    srv := httptest.NewServer(mux)
    t.Cleanup(srv.Close)
    return srv, mgr
}

func httpGet(t *testing.T, url string) (*http.Response, []byte) {
    t.Helper()
    req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
    if err != nil { t.Fatalf("new req: %v", err) }
    resp, err := http.DefaultClient.Do(req)
    if err != nil { t.Fatalf("do req: %v", err) }
    body, _ := io.ReadAll(resp.Body)
    _ = resp.Body.Close()
    return resp, body
}

func httpPostJSON(t *testing.T, url string, payload []byte) (*http.Response, []byte) {
    t.Helper()
    req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payload))
    if err != nil { t.Fatalf("new req: %v", err) }
    req.Header.Set("Content-Type", "application/json")
    resp, err := http.DefaultClient.Do(req)
    if err != nil { t.Fatalf("do req: %v", err) }
    body, _ := io.ReadAll(resp.Body)
    _ = resp.Body.Close()
    return resp, body
}
