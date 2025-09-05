package manager

import (
	"testing"
	"time"
)

func TestEnsureInstance_FastPathUpdatesLastUsed(t *testing.T) {
	m := NewWithConfig(ManagerConfig{MaxQueueDepth: 1})
	// Seed an instance that's already ready
	m.mu.Lock()
	inst := &Instance{ID: "m", State: StateReady, LastUsed: time.Unix(1, 0), genCh: make(chan struct{}, 1), queueCh: make(chan struct{}, 1)}
	m.instances["m"] = inst
	m.mu.Unlock()
	before := inst.LastUsed
	if err := m.EnsureInstance(testCtx(t), "m"); err != nil {
		t.Fatalf("EnsureInstance fast path: %v", err)
	}
	m.mu.RLock()
	after := m.instances["m"].LastUsed
	m.mu.RUnlock()
	if !after.After(before) {
		t.Fatalf("expected LastUsed to be updated; before=%v after=%v", before, after)
	}
}
