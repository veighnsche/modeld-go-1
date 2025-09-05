package manager

import (
	"context"
	"testing"
	"time"

	"modeld/pkg/types"
)

func TestSwitch_BackgroundWarmupContinuesAfterCancel(t *testing.T) {
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: "m.gguf"}}})
	// Cancel the context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	op, err := m.Switch(ctx, "m")
	if err != nil || op == "" {
		t.Fatalf("Switch returned error/op empty: op=%q err=%v", op, err)
	}
	// Wait briefly to allow background ensure to run
	time.Sleep(80 * time.Millisecond)
	m.mu.RLock()
	inst := m.instances["m"]
	m.mu.RUnlock()
	if inst == nil {
		t.Fatalf("expected instance to be created by background warmup")
	}
}
