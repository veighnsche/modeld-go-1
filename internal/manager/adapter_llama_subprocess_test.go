package manager

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"modeld/pkg/types"
)

// buildTestBinary builds the fake llama server used for subprocess tests and returns its path.
func buildTestBinary(t *testing.T) string {
	t.Helper()
	tdir := t.TempDir()
	bin := filepath.Join(tdir, "fake_llama_server")
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/fake_llama_server.go")
	cmd.Dir = "." // package dir internal/manager
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake server: %v: %s", err, string(out))
	}
	return bin
}

func TestSubprocessEnsureAndStop(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	bin := buildTestBinary(t)
	cfg := ManagerConfig{
		Registry:       []types.Model{{ID: "m1", Path: "m1.gguf"}},
		DefaultModel:   "m1",
		SpawnLlama:     true,
		LlamaBin:       bin,
		LlamaHost:      "127.0.0.1",
		LlamaPortStart: 31000,
		LlamaPortEnd:   31010,
	}
	m := NewWithConfig(cfg)
	// Ensure instance spawns and becomes ready
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.EnsureInstance(ctx, "m1"); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	// Validate instance fields
	m.mu.RLock()
	inst := m.instances["m1"]
	m.mu.RUnlock()
	if inst == nil || inst.State != StateReady {
		t.Fatalf("instance not ready: %+v", inst)
	}
	if inst.Port <= 0 || inst.PID <= 0 {
		t.Fatalf("expected port and pid to be set, got port=%d pid=%d", inst.Port, inst.PID)
	}
	// Stop via adapter and ensure removal
	if sa, ok := m.adapter.(*llamaSubprocessAdapter); ok {
		if err := sa.Stop("m1.gguf"); err != nil {
			t.Fatalf("Stop: %v", err)
		}
	}
}

func TestSubprocessAccessorRace(t *testing.T) {
	// This test exercises concurrent access patterns; run with -race.
	bin := buildTestBinary(t)
	cfg := ManagerConfig{
		Registry:       []types.Model{{ID: "m2", Path: "m2.gguf"}},
		SpawnLlama:     true,
		LlamaBin:       bin,
		LlamaHost:      "127.0.0.1",
		LlamaPortStart: 31011,
		LlamaPortEnd:   31020,
	}
	m := NewWithConfig(cfg)
	sa, ok := m.adapter.(*llamaSubprocessAdapter)
	if !ok {
		t.Fatalf("expected subprocess adapter")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = m.EnsureInstance(ctx, "m2")
		close(done)
	}()
	// Busy read accessor while EnsureInstance runs
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, _, _, _ = sa.getProcInfo("m2.gguf")
		runtime.Gosched()
	}
	<-done
}
