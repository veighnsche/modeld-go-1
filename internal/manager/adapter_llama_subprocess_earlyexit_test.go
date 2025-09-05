//go:build integration
// +build integration

package manager

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"modeld/pkg/types"
)

func buildExitBinary(t *testing.T) string {
	t.Helper()
	tdir := t.TempDir()
	bin := filepath.Join(tdir, "exit_1")
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/exit_1.go")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build exit_1: %v: %s", err, string(out))
	}
	return bin
}

func TestSubprocessEarlyExitEmitsError(t *testing.T) {
	bin := buildExitBinary(t)
	sa := NewLlamaSubprocessAdapter(ManagerConfig{LlamaBin: bin, SpawnLlama: true}).(*llamaSubprocessAdapter)
	pub := NewMemoryPublisher()
	sa.setPublisher(pub)
	_, err := sa.ensureProcess("m.gguf")
	if err == nil {
		t.Fatalf("expected error due to early exit")
	}
	// Expect spawn_start and spawn_exit events
	events := pub.Events()
	var startOK, exitOK bool
	for _, e := range events {
		if e.Name == "spawn_start" { startOK = true }
		if e.Name == "spawn_exit" { exitOK = true }
	}
	if !startOK || !exitOK {
		t.Fatalf("expected spawn_start and spawn_exit events, got: %+v", events)
	}
}

func TestStopAllInstancesRemovesEntries(t *testing.T) {
	bin := buildTestBinary(t) // from adapter_llama_subprocess_test.go
	cfg := ManagerConfig{
		Registry:       []types.Model{{ID: "m1", Path: "m1.gguf"}},
		DefaultModel:   "m1",
		SpawnLlama:     true,
		LlamaBin:       bin,
		LlamaHost:      "127.0.0.1",
		LlamaPortStart: 31200,
		LlamaPortEnd:   31210,
	}
	m := NewWithConfig(cfg)
	// Ensure spawns an instance
	if err := m.EnsureInstance(testContext(t), "m1"); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	m.StopAllInstances()
	// Map should be empty in adapter
	if sa, ok := m.adapter.(*llamaSubprocessAdapter); ok {
		sa.mu.Lock()
		defer sa.mu.Unlock()
		if len(sa.procs) != 0 {
			t.Fatalf("expected procs to be empty after StopAllInstances, got %d", len(sa.procs))
		}
	}
}

// testContext returns a cancellable context that will be canceled by the test cleanup.
func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
