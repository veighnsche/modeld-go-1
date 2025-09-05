package manager

import (
	"errors"
	"testing"
	"time"

	"modeld/pkg/types"
)

func TestEvictUntilFits_ReturnsBudgetExceededWhenNoIdleAndDoesNotFit(t *testing.T) {
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: "m.gguf"}}, DefaultModel: "m"})
	// Configure a very tight budget so any new requirement won't fit
	m.mu.Lock()
	m.budgetMB = 1
	m.marginMB = 0
	// Seed a single busy instance so it's not idle (has queue/inflight)
	inst := &Instance{ID: "m", State: StateReady, LastUsed: time.Now(), EstVRAMMB: 1, genCh: make(chan struct{}, 1), queueCh: make(chan struct{}, 1)}
	inst.genCh <- struct{}{} // mark in-flight so it's non-idle
	m.instances["m"] = inst
	m.mu.Unlock()
	// Ask to evict until fits with requiredMB > budget, no idle instances present
	err := m.evictUntilFits(10)
	if err == nil || !IsBudgetExceeded(err) {
		t.Fatalf("expected budget exceeded error, got %v", err)
	}
	// Also ensure it's not classified as dependency unavailable
	if IsDependencyUnavailable(err) {
		t.Fatalf("should not be dependency unavailable")
	}
}
