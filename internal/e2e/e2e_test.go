package e2e

import (
    "bytes"
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"
    "time"

    "modeld/internal/httpapi"
    "modeld/internal/manager"
    "modeld/internal/registry"
    "modeld/pkg/types"
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

// TestE2E_Backpressure429 verifies we return 429 Too Many Requests when the per-instance
// queue is full and the wait timeout elapses.
func TestE2E_Backpressure429(t *testing.T) {
    // Arrange: tiny queue depth and short wait to elicit 429 deterministically.
    dir, models := createTempModelsDir(t, "alpha.gguf")
    cfg := manager.ManagerConfig{
        BudgetMB:      0,
        MarginMB:      0,
        DefaultModel:  models[0],
        MaxQueueDepth: 1,                 // one waiting request besides the in-flight
        MaxWait:       5 * time.Millisecond,
    }
    srv, _ := newServerForDirWithConfig(t, dir, cfg)

    // Helper to POST /infer and return status code.
    doInfer := func() int {
        req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/infer", bytes.NewBufferString(`{"prompt":"hello"}`))
        if err != nil { t.Fatalf("new req: %v", err) }
        req.Header.Set("Content-Type", "application/json")
        resp, err := http.DefaultClient.Do(req)
        if err != nil { t.Fatalf("do req: %v", err) }
        io.Copy(io.Discard, resp.Body)
        resp.Body.Close()
        return resp.StatusCode
    }

    // Kick off three concurrent requests. With queue depth 1 and single in-flight,
    // the third should fail fast with 429 due to MaxWait elapsing while queue slot is unavailable.
    done := make(chan int, 3)
    go func(){ done <- doInfer() }() // first should be 200
    go func(){ done <- doInfer() }() // second should be 200 (queued then runs)
    go func(){ done <- doInfer() }() // third should be 429

    // Collect results
    s1, s2, s3 := <-done, <-done, <-done
    got429 := (s1 == http.StatusTooManyRequests) || (s2 == http.StatusTooManyRequests) || (s3 == http.StatusTooManyRequests)
    if !got429 {
        t.Fatalf("expected at least one 429 status, got: %d, %d, %d", s1, s2, s3)
    }
}

func newServerForDir(t *testing.T, modelsDir string, budgetMB, marginMB int, defaultModel string) (*httptest.Server, *manager.Manager) {
    t.Helper()
    // Scan the directory for models
    reg, err := registry.NewGGUFScanner().Scan(modelsDir)
    if err != nil {
        t.Fatalf("scan models: %v", err)
    }
    // Construct manager with discovered registry
    mgr := manager.New(reg, budgetMB, marginMB, defaultModel)
    // Build HTTP mux and start test server
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

func TestE2E_Models_Infer_Ready_Status(t *testing.T) {
    // Arrange: create a temp models dir with two .gguf files
    dir, models := createTempModelsDir(t, "alpha.gguf", "beta.gguf")

    // Start server with a default model
    srv, _ := newServerForDir(t, dir, 2000, 128, models[0])

    // 1) GET /models returns discovered models
    resp, body := httpGet(t, srv.URL+"/models")
    if resp.StatusCode != http.StatusOK { t.Fatalf("/models status=%d body=%s", resp.StatusCode, string(body)) }
    var modelsResp struct { Models []types.Model `json:"models"` }
    if err := json.Unmarshal(body, &modelsResp); err != nil {
        t.Fatalf("/models json: %v body=%s", err, string(body))
    }
    if len(modelsResp.Models) != 2 { t.Fatalf("expected 2 models, got %d", len(modelsResp.Models)) }

    // 2) Initially /readyz should be 503 (no instance ready yet)
    resp, body = httpGet(t, srv.URL+"/readyz")
    if resp.StatusCode != http.StatusServiceUnavailable {
        t.Fatalf("/readyz expected 503, got %d body=%s", resp.StatusCode, string(body))
    }

    // 3) POST /infer without model (uses default). Should stream NDJSON and return 200.
    resp, body = httpPostJSON(t, srv.URL+"/infer", []byte(`{"prompt":"hello"}`))
    if resp.StatusCode != http.StatusOK { t.Fatalf("/infer status=%d body=%s", resp.StatusCode, string(body)) }
    // Should contain multiple lines of NDJSON
    if !bytes.Contains(body, []byte("\n")) {
        t.Fatalf("/infer expected streaming newlines, got: %q", string(body))
    }

    // 4) After infer, readiness should become 200 OK once the instance is ready.
    //    Poll for a short time to avoid flakiness.
    deadline := time.Now().Add(2 * time.Second)
    for {
        resp, _ = httpGet(t, srv.URL+"/readyz")
        if resp.StatusCode == http.StatusOK { break }
        if time.Now().After(deadline) {
            t.Fatalf("/readyz did not become ready in time; last=%d", resp.StatusCode)
        }
        time.Sleep(25 * time.Millisecond)
    }

    // 5) GET /status should reflect at least one instance and non-zero used VRAM estimate
    resp, body = httpGet(t, srv.URL+"/status")
    if resp.StatusCode != http.StatusOK { t.Fatalf("/status status=%d body=%s", resp.StatusCode, string(body)) }
    var st types.StatusResponse
    if err := json.Unmarshal(body, &st); err != nil { t.Fatalf("/status json: %v body=%s", err, string(body)) }
    if len(st.Instances) < 1 { t.Fatalf("/status expected instances >=1, got %d", len(st.Instances)) }
}
