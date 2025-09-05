package manager

import (
	"context"
	"testing"
	"time"

	"modeld/pkg/types"
)

// Covers the ctx.Done branch when attempting to reserve a queue slot.
func TestBeginGeneration_CancelBeforeQueue(t *testing.T) {
	// queue depth 0 so the send on queueCh blocks and select can pick ctx.Done
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: p}}, DefaultModel: "m", MaxQueueDepth: 0, MaxWait: 200 * time.Millisecond})
	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.beginGeneration(ctx, "m"); err == nil {
		t.Fatalf("expected error on canceled context")
	}
}

// Covers the ctx.Done branch while waiting for genCh after queue slot reserved.
func TestBeginGeneration_CancelWhileWaitingForGen(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: p}}, DefaultModel: "m", MaxQueueDepth: 1, MaxWait: 500 * time.Millisecond})
	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	// Saturate genCh to force blocking on second phase
	m.mu.RLock()
	inst := m.instances["m"]
	m.mu.RUnlock()
	inst.genCh <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// cancel shortly after to hit ctx.Done case
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	if _, err := m.beginGeneration(ctx, "m"); err == nil {
		t.Fatalf("expected error due to canceled context while waiting for gen slot")
	}
	// cleanup the token placed into genCh
	<-inst.genCh
}
