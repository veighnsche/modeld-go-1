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
