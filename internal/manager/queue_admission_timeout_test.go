package manager

import (
	"context"
	"testing"
	"time"

	"modeld/pkg/types"
)

func TestBeginGeneration_QueueTimeout(t *testing.T) {
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: "m.gguf"}}, DefaultModel: "m", MaxQueueDepth: 1, MaxWait: 20 * time.Millisecond})
	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	// First acquire to occupy both queue and gen slots
	rel, err := m.beginGeneration(context.Background(), "m")
	if err != nil {
		t.Fatalf("beginGeneration first: %v", err)
	}
	defer rel()
	// Second should timeout on queue slot (since depth=1)
	_, err = m.beginGeneration(context.Background(), "m")
	if err == nil || !IsTooBusy(err) {
		t.Fatalf("expected tooBusyError, got %v", err)
	}
}

func TestBeginGeneration_GenTimeout(t *testing.T) {
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: "m.gguf"}}, DefaultModel: "m", MaxQueueDepth: 2, MaxWait: 20 * time.Millisecond})
	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	// Occupy genCh so acquisitions will block at gen stage
	m.mu.Lock()
	inst := m.instances["m"]
	inst.genCh <- struct{}{}
	m.mu.Unlock()
	// Should acquire queue slot, then timeout on gen slot resulting in tooBusy
	_, err := m.beginGeneration(context.Background(), "m")
	if err == nil || !IsTooBusy(err) {
		t.Fatalf("expected tooBusyError on gen wait, got %v", err)
	}
}
