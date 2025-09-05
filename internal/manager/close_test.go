package manager

import (
	"testing"
	"time"

	"modeld/pkg/types"
)

func TestManagerCloseStopsInstances(t *testing.T) {
	bin := buildTestBinary(t)
	cfg := ManagerConfig{
		Registry:       []types.Model{{ID: "m", Path: "m.gguf"}},
		DefaultModel:   "m",
		SpawnLlama:     true,
		LlamaBin:       bin,
		LlamaHost:      "127.0.0.1",
		LlamaPortStart: 31300,
		LlamaPortEnd:   31310,
		DrainTimeout:   100 * time.Millisecond,
	}
	m := NewWithConfig(cfg)
	if err := m.EnsureInstance(testCtx(t), "m"); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if sa, ok := m.adapter.(*llamaSubprocessAdapter); ok {
		sa.mu.Lock()
		defer sa.mu.Unlock()
		if len(sa.procs) != 0 {
			t.Fatalf("expected no procs after Close, got %d", len(sa.procs))
		}
	}
}
