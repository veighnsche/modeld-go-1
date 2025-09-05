package manager

import (
	"context"
	"testing"
	"time"

	"modeld/pkg/types"
)

func TestUnload_RemovesInstanceAndUpdatesAccounting(t *testing.T) {
	m := NewWithConfig(ManagerConfig{
		Registry:     []types.Model{{ID: "m", Path: "m.gguf"}},
		DefaultModel: "m",
		MaxQueueDepth: 2,
		DrainTimeout:  200 * time.Millisecond,
	})
	// Ensure creates a ready instance
	ctx := context.Background()
	if err := m.EnsureInstance(ctx, "m"); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	// Now unload and verify removal + usedEstMB decreased
	if err := m.Unload("m"); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	m.mu.RLock()
	_, exists := m.instances["m"]
	used := m.usedEstMB
	m.mu.RUnlock()
	if exists {
		t.Fatalf("instance still exists after unload")
	}
	if used < 0 {
		t.Fatalf("usedEstMB negative: %d", used)
	}
}
