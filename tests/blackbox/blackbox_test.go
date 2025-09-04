package blackbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// findFreePort picks an available TCP port on localhost.
func findFreePort(t *testing.T) (int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatalf("listen: %v", err) }
	addr := ln.Addr().String()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil { t.Fatalf("split: %v", err) }
	cleanup := func(){ _ = ln.Close() }
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return port, cleanup
}

func projectRootFromThisFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("runtime.Caller failed") }
	// this file: <root>/tests/blackbox/blackbox_test.go
	bbDir := filepath.Dir(thisFile)
	root := filepath.Dir(filepath.Dir(bbDir))
	return root
}

func buildBinary(t *testing.T) string {
	t.Helper()
	root := projectRootFromThisFile(t)
	outDir := t.TempDir()
	binPath := filepath.Join(outDir, "modeld")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/modeld")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(out))
	}
	return binPath
}

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

type serverProc struct {
	cmd  *exec.Cmd
	base string // http base URL, e.g. http://127.0.0.1:18080
}

func startServer(t *testing.T, bin string, modelsDir string, defaultModel string, port int) *serverProc {
	t.Helper()
	addr := fmt.Sprintf(":%d", port)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	args := []string{
		"--addr", addr,
		"--models-dir", modelsDir,
	}
	if defaultModel != "" {
		args = append(args, "--default-model", defaultModel)
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	// Wait for healthz
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK { break }
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("server did not become healthy in time")
		}
		time.Sleep(50 * time.Millisecond)
	}
	sp := &serverProc{cmd: cmd, base: base}
	t.Cleanup(func(){ _ = cmd.Process.Kill() })
	return sp
}

func get(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil { t.Fatalf("new req: %v", err) }
	resp, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatalf("do: %v", err) }
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, b
}

func postJSON(t *testing.T, url string, payload []byte) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil { t.Fatalf("new req: %v", err) }
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatalf("do: %v", err) }
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, b
}

func TestBlackbox_Flow(t *testing.T) {
	// Build server binary
	bin := buildBinary(t)
	// Create models
	modelsDir, models := createTempModelsDir(t, "alpha.gguf", "beta.gguf")
	// Reserve a free port, then release listener before starting the server
	port, release := findFreePort(t)
	release()
	// Start server with default model
	sp := startServer(t, bin, modelsDir, models[0], port)

	// /healthz
	resp, body := get(t, sp.base+"/healthz")
	if resp.StatusCode != http.StatusOK { t.Fatalf("/healthz %d %s", resp.StatusCode, string(body)) }

	// /models
	resp, body = get(t, sp.base+"/models")
	if resp.StatusCode != http.StatusOK { t.Fatalf("/models %d %s", resp.StatusCode, string(body)) }
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") { t.Fatalf("/models content-type=%s", ct) }
	var modelsResp struct{ Models []struct{ ID string `json:"id"` } `json:"models"` }
	if err := json.Unmarshal(body, &modelsResp); err != nil { t.Fatalf("/models json: %v body=%s", err, string(body)) }
	if len(modelsResp.Models) != 2 { t.Fatalf("expected 2 models, got %d", len(modelsResp.Models)) }

	// /readyz initially 503
	resp, body = get(t, sp.base+"/readyz")
	if resp.StatusCode != http.StatusServiceUnavailable { t.Fatalf("/readyz initial %d %s", resp.StatusCode, string(body)) }

	// /infer without model uses default
	resp, body = postJSON(t, sp.base+"/infer", []byte(`{"prompt":"hello"}`))
	if resp.StatusCode != http.StatusOK { t.Fatalf("/infer %d %s", resp.StatusCode, string(body)) }
	if !bytes.Contains(body, []byte("\n")) { t.Fatalf("/infer expected newline-delimited chunks, got: %q", string(body)) }

	// /readyz eventually 200
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, _ = get(t, sp.base+"/readyz")
		if resp.StatusCode == http.StatusOK { break }
		if time.Now().After(deadline) { t.Fatalf("/readyz did not become ready in time; last=%d", resp.StatusCode) }
		time.Sleep(25 * time.Millisecond)
	}

	// /status shows at least one instance
	resp, body = get(t, sp.base+"/status")
	if resp.StatusCode != http.StatusOK { t.Fatalf("/status %d %s", resp.StatusCode, string(body)) }
	var statusResp struct{ Instances []any `json:"instances"` }
	if err := json.Unmarshal(body, &statusResp); err != nil { t.Fatalf("/status json: %v body=%s", err, string(body)) }
	if len(statusResp.Instances) < 1 { t.Fatalf("expected instances >=1, got %d", len(statusResp.Instances)) }
}

func TestBlackbox_Infer_ModelNotFound_404(t *testing.T) {
	bin := buildBinary(t)
	modelsDir, _ := createTempModelsDir(t, "alpha.gguf")
	port, release := findFreePort(t)
	release()
	sp := startServer(t, bin, modelsDir, "alpha.gguf", port)

	resp, body := postJSON(t, sp.base+"/infer", []byte(`{"model":"missing.gguf","prompt":"hi"}`))
	if resp.StatusCode != http.StatusNotFound { t.Fatalf("expected 404, got %d, body=%s", resp.StatusCode, string(body)) }
}

func TestBlackbox_Infer_NoDefault_NoModel_404(t *testing.T) {
	bin := buildBinary(t)
	modelsDir, _ := createTempModelsDir(t, "alpha.gguf")
	port, release := findFreePort(t)
	release()
	sp := startServer(t, bin, modelsDir, "", port)

	resp, body := postJSON(t, sp.base+"/infer", []byte(`{"prompt":"hi"}`))
	if resp.StatusCode != http.StatusNotFound { t.Fatalf("expected 404, got %d, body=%s", resp.StatusCode, string(body)) }
}
