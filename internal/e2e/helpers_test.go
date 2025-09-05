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
	mock := startMockLlamaServer(t, defaultModel)
	cfg := manager.ManagerConfig{
		Registry:           reg,
		BudgetMB:           budgetMB,
		MarginMB:           marginMB,
		DefaultModel:       defaultModel,
		MaxQueueDepth:      0,
		MaxWait:            0,
		LlamaServerURL:     mock.URL,
		LlamaAPIKey:        "",
		LlamaRequestTimeout: 5 * 1e9, // 5s
		LlamaConnectTimeout: 1 * 1e9, // 1s
		LlamaUseOpenAI:     true,
	}
	mgr := manager.NewWithConfig(cfg)
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
	if cfg.DefaultModel == "" && len(reg) > 0 {
		cfg.DefaultModel = reg[0].ID
	}
	// Ensure a mock llama server is set up for tests
	if cfg.LlamaServerURL == "" {
		mock := startMockLlamaServer(t, cfg.DefaultModel)
		cfg.LlamaServerURL = mock.URL
		cfg.LlamaUseOpenAI = true
		if cfg.LlamaRequestTimeout == 0 { cfg.LlamaRequestTimeout = 5 * 1e9 }
		if cfg.LlamaConnectTimeout == 0 { cfg.LlamaConnectTimeout = 1 * 1e9 }
	}
	mgr := manager.NewWithConfig(cfg)
	mux := httpapi.NewMux(mgr)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, mgr
}

func httpGet(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new req: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do req: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, body
}

func httpPostJSON(t *testing.T, url string, payload []byte) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new req: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do req: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, body
}

// startMockLlamaServer returns a test server emulating minimal llama.cpp OpenAI endpoints.
func startMockLlamaServer(t *testing.T, modelID string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	// GET /v1/models -> { data: [{id: modelID}] }
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := struct{ Data []struct{ ID string `json:"id"` } `json:"data"` }{}
		resp.Data = []struct{ ID string `json:"id"` }{{ID: modelID}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	// POST /v1/completions -> stream SSE fragments then [DONE]
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		// Two small chunks then DONE
		write := func(s string) {
			_, _ = w.Write([]byte(s))
			_, _ = w.Write([]byte("\n"))
			if f, ok := w.(http.Flusher); ok { f.Flush() }
		}
		// minimal chunk JSON
		mk := func(s string) string {
			msg := struct {
				Object  string `json:"object"`
				Choices []struct{ Delta struct{ Content string `json:"content"` } `json:"delta"`; FinishReason string `json:"finish_reason"` } `json:"choices"`
			}{Object: "text_completion.chunk"}
			msg.Choices = make([]struct{ Delta struct{ Content string `json:"content"` } `json:"delta"`; FinishReason string `json:"finish_reason"` }, 1)
			msg.Choices[0].Delta.Content = s
			b, _ := json.Marshal(msg)
			return "data: " + string(b)
		}
		// Simulate generation time to trigger backpressure tests
		time.Sleep(50 * time.Millisecond)
		write(mk("hi"))
		time.Sleep(50 * time.Millisecond)
		write(mk(" there"))
		write("data: [DONE]")
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}
