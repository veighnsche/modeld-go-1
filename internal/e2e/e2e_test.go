package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"modeld/internal/httpapi"
	"modeld/internal/manager"
	"modeld/pkg/types"
)

// Helpers are defined in helpers_test.go

// TestE2E_Config_CORS_And_InferTimeout ensures the mux honors package-level
// httpapi configuration for CORS and infer timeout behavior.
func TestE2E_Config_CORS_And_InferTimeout(t *testing.T) {
	dir, models := createTempModelsDir(t, "alpha.gguf")
	// Set package-level options
	httpapi.SetCORSOptions(true, []string{"http://example.com"}, []string{"GET", "POST", "OPTIONS"}, []string{"Content-Type"})
	httpapi.SetInferTimeoutSeconds(1) // very small timeout; our stub infer should still complete in time

	// Spin up server
	srv, _ := newServerForDir(t, dir, 0, 0, models[0])

	// Preflight request should include CORS headers
	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/infer", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight err: %v", err)
	}
    _ = resp.Body.Close()
    if ao := resp.Header.Get("Access-Control-Allow-Origin"); ao == "" {
        t.Fatalf("expected CORS allow origin header, got none")
    }

    // Normal infer should still succeed
    r, body := httpPostJSON(t, srv.URL+"/infer", []byte(`{"prompt":"hello"}`))
    if r.StatusCode != http.StatusOK {
        t.Fatalf("infer status=%d body=%s", r.StatusCode, string(body))
    }
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
		MaxQueueDepth: 1, // one waiting request besides the in-flight
		MaxWait:       1 * time.Millisecond,
	}
	srv, _ := newServerForDirWithConfig(t, dir, cfg)

	// Helper to POST /infer and return status code.
	doInfer := func() int {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/infer", bytes.NewBufferString(`{"prompt":"hello"}`))
		if err != nil {
			t.Fatalf("new req: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do req: %v", err)
		}
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			t.Fatalf("io.Copy: %v", err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	// Kick off three concurrent requests. With queue depth 1 and single in-flight,
	// the third should fail fast with 429 due to MaxWait elapsing while queue slot is unavailable.
	done := make(chan int, 3)
	go func() { done <- doInfer() }() // first should be 200
	go func() { done <- doInfer() }() // second should be 200 (queued then runs)
	go func() { done <- doInfer() }() // third should be 429

	// Collect results
	s1, s2, s3 := <-done, <-done, <-done
	got429 := (s1 == http.StatusTooManyRequests) || (s2 == http.StatusTooManyRequests) || (s3 == http.StatusTooManyRequests)
	if !got429 {
		t.Fatalf("expected at least one 429 status, got: %d, %d, %d", s1, s2, s3)
	}
}

// newServerForDir and newServerForDirWithConfig are provided by helpers_test.go

// httpGet and httpPostJSON are provided by helpers_test.go

func TestE2E_Models_Infer_Ready_Status(t *testing.T) {
	// Arrange: create a temp models dir with two .gguf files
	dir, models := createTempModelsDir(t, "alpha.gguf", "beta.gguf")

	// Start server with a default model
	srv, _ := newServerForDir(t, dir, 2000, 128, models[0])

	// 1) GET /models returns discovered models
	resp, body := httpGet(t, srv.URL+"/models")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/models status=%d body=%s", resp.StatusCode, string(body))
	}
	var modelsResp struct {
		Models []types.Model `json:"models"`
	}
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		t.Fatalf("/models json: %v body=%s", err, string(body))
	}
	if len(modelsResp.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(modelsResp.Models))
	}

	// 2) Initially /readyz should be 503 (no instance ready yet)
	resp, body = httpGet(t, srv.URL+"/readyz")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("/readyz expected 503, got %d body=%s", resp.StatusCode, string(body))
	}

	// 3) POST /infer without model (uses default). Should stream NDJSON and return 200.
	resp, body = httpPostJSON(t, srv.URL+"/infer", []byte(`{"prompt":"hello"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/infer status=%d body=%s", resp.StatusCode, string(body))
	}
	// Should contain multiple lines of NDJSON
	if !bytes.Contains(body, []byte("\n")) {
		t.Fatalf("/infer expected streaming newlines, got: %q", string(body))
	}

	// 4) After infer, readiness should become 200 OK once the instance is ready.
	//    Poll for a short time to avoid flakiness.
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, _ = httpGet(t, srv.URL+"/readyz")
		if resp.StatusCode == http.StatusOK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("/readyz did not become ready in time; last=%d", resp.StatusCode)
		}
		time.Sleep(25 * time.Millisecond)
	}

	// 5) GET /status should reflect at least one instance and non-zero used VRAM estimate
	resp, body = httpGet(t, srv.URL+"/status")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/status status=%d body=%s", resp.StatusCode, string(body))
	}
	var st types.StatusResponse
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("/status json: %v body=%s", err, string(body))
	}
	if len(st.Instances) < 1 {
		t.Fatalf("/status expected instances >=1, got %d", len(st.Instances))
	}
}

// TestE2E_Metrics_Endpoint ensures Prometheus metrics endpoint is exposed
// and returns standard modeld http metrics after at least one request.
func TestE2E_Metrics_Endpoint(t *testing.T) {
	dir, models := createTempModelsDir(t, "alpha.gguf")
	srv, _ := newServerForDir(t, dir, 0, 0, models[0])

	// Trigger at least one request so counters are non-zero
	r, body := httpPostJSON(t, srv.URL+"/infer", []byte(`{"prompt":"hello"}`))
	if r.StatusCode != http.StatusOK {
		t.Fatalf("/infer status=%d body=%s", r.StatusCode, string(body))
	}

	// Fetch /metrics
	resp, mbody := httpGet(t, srv.URL+"/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/metrics status=%d body=%s", resp.StatusCode, string(mbody))
	}
	// Look for a known metric name
	if !bytes.Contains(mbody, []byte("modeld_http_requests_total")) {
		t.Fatalf("/metrics missing expected counter; got: %q", string(mbody[:min(200, len(mbody))]))
	}
}

// min helper for small debug slices
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestE2E_RealHaiku_IfLlamaAvailable exercises a real LLM end-to-end flow without mocks.
// It will be skipped unless the environment variable LLAMA_URL is set to a reachable
// llama.cpp server (OpenAI-compatible /v1/completions). Optionally, LLAMA_API_KEY can be
// set for authentication.
func TestE2E_RealHaiku_IfLlamaAvailable(t *testing.T) {
    llamaURL := strings.TrimSpace(os.Getenv("LLAMA_URL"))
    if llamaURL == "" {
        t.Skip("LLAMA_URL not set; skipping real haiku e2e test")
    }
    // Prepare a minimal registry with a placeholder model file; the adapter uses server mode.
    dir, models := createTempModelsDir(t, "placeholder.gguf")

    cfg := manager.ManagerConfig{
        BudgetMB:            0,
        MarginMB:            0,
        DefaultModel:        models[0],
        MaxQueueDepth:       2,
        MaxWait:             10 * time.Second,
        LlamaServerURL:      llamaURL,
        LlamaAPIKey:         os.Getenv("LLAMA_API_KEY"),
        LlamaRequestTimeout: 30 * time.Second,
        LlamaConnectTimeout: 5 * time.Second,
        LlamaUseOpenAI:      true,
    }
    srv, _ := newServerForDirWithConfig(t, dir, cfg)

    // Quick health check
    if r, _ := httpGet(t, srv.URL+"/healthz"); r.StatusCode != http.StatusOK {
        t.Skipf("server not healthy; status=%d", r.StatusCode)
    }

    // Ask for a haiku and stream NDJSON
    prompt := "Write a 3-line haiku about the ocean."
    resp, body := httpPostJSON(t, srv.URL+"/infer", []byte(fmt.Sprintf(`{"prompt":%q,"max_tokens":128,"temperature":0.7,"top_p":0.95}`, prompt)))
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("/infer status=%d body=%s", resp.StatusCode, string(body))
    }
    // Parse NDJSON lines and extract tokens/final content
    lines := strings.Split(string(body), "\n")
    var tokens []string
    var finalContent string
    for _, ln := range lines {
        s := strings.TrimSpace(ln)
        if s == "" {
            continue
        }
        var obj map[string]any
        if err := json.Unmarshal([]byte(s), &obj); err != nil {
            continue
        }
        if tok, ok := obj["token"].(string); ok && tok != "" {
            tokens = append(tokens, tok)
        }
        if done, _ := obj["done"].(bool); done {
            if c, ok := obj["content"].(string); ok {
                finalContent = c
            }
        }
    }
    content := strings.TrimSpace(func() string {
        if finalContent != "" {
            return finalContent
        }
        return strings.Join(tokens, "")
    }())
    // Print the haiku to test logs as proof of generation
    if content != "" {
        t.Logf("\n----- GENERATED HAIKU -----\n%s\n---------------------------\n", content)
        // Duplicate to stdout for environments that do not show t.Log without -v
        fmt.Printf("\n----- GENERATED HAIKU -----\n%s\n---------------------------\n", content)
    }
    if content == "" {
        t.Fatalf("expected non-empty haiku content")
    }
}
