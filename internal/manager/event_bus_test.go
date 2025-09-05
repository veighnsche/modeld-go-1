package manager

import (
	"context"
	"testing"
	"time"

	"modeld/pkg/types"
)

func TestEventPublisher_EnsureAndUnload_EmitsEvents(t *testing.T) {
	m := NewWithConfig(ManagerConfig{
		Registry:     []types.Model{{ID: "m", Path: "m.gguf"}},
		DefaultModel: "m",
		DrainTimeout: 50 * time.Millisecond,
	})
	pub := NewMemoryPublisher()
	m.SetEventPublisher(pub)
	// Ensure triggers ensure_* events (without spawn adapter path)
	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	// Unload triggers unload_* events
	if err := m.Unload("m"); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	evts := pub.Events()
	// Make sure at least these events occurred in some order
	want := map[string]bool{
		"ensure_start":  false,
		"ensure_ready":  false,
		"unload_start":  false,
		"unload_done":   false,
	}
	for _, e := range evts {
		if _, ok := want[e.Name]; ok {
			want[e.Name] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Fatalf("expected event %q to be published; got events: %+v", k, evts)
		}
	}
}
