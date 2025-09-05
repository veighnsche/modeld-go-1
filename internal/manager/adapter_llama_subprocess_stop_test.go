package manager

import (
	"testing"

	"modeld/pkg/types"
)

func TestSubprocessStopDirectRemovesProc(t *testing.T) {
	bin := buildTestBinary(t)
	cfg := ManagerConfig{
		Registry:       []types.Model{{ID: "m", Path: "m.gguf"}},
		DefaultModel:   "m",
		SpawnLlama:     true,
		LlamaBin:       bin,
		LlamaHost:      "127.0.0.1",
		LlamaPortStart: 31400,
		LlamaPortEnd:   31410,
	}
	m := NewWithConfig(cfg)
	if err := m.EnsureInstance(testCtx(t), "m"); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	if sa, ok := m.adapter.(*llamaSubprocessAdapter); ok {
		// call Stop on model path
		_ = sa.Stop("m.gguf")
		sa.mu.Lock()
		defer sa.mu.Unlock()
		if len(sa.procs) != 0 {
			t.Fatalf("expected procs to be empty after Stop, got %d", len(sa.procs))
		}
	}
}
