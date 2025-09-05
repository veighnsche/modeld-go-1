package manager

import (
	"reflect"
	"testing"
)

func TestNewWithConfig_AdapterSelectionAndDefaults(t *testing.T) {
	// Server adapter selected when LlamaServerURL is set
	m := NewWithConfig(ManagerConfig{LlamaServerURL: "http://127.0.0.1:8081"})
	if _, ok := m.adapter.(*llamaServerAdapter); !ok {
		t.Fatalf("expected server adapter when LlamaServerURL is set")
	}
	// Spawn adapter selected when SpawnLlama and LlamaBin set
	m2 := NewWithConfig(ManagerConfig{SpawnLlama: true, LlamaBin: "/bin/true"})
	if _, ok := m2.adapter.(*llamaSubprocessAdapter); !ok {
		t.Fatalf("expected subprocess adapter when SpawnLlama and LlamaBin set")
	}
	// Defaults: MaxQueueDepth, MaxWait, DrainTimeout
	m3 := NewWithConfig(ManagerConfig{})
	if m3.maxQueueDepth == 0 || m3.maxWait == 0 || m3.drainTimeout == 0 {
		t.Fatalf("expected defaults to be applied, got depth=%d wait=%s drain=%s", m3.maxQueueDepth, m3.maxWait, m3.drainTimeout)
	}
}

func TestManager_SetEventPublisher_WiresAdapter(t *testing.T) {
	m := NewWithConfig(ManagerConfig{SpawnLlama: true, LlamaBin: "/bin/true"})
	pub := NewMemoryPublisher()
	m.SetEventPublisher(pub)
	if sa, ok := m.adapter.(*llamaSubprocessAdapter); ok {
		// reflect that publisher is set (not nil)
		if reflect.ValueOf(sa.publisher).IsZero() {
			t.Fatalf("expected adapter publisher to be set")
		}
	} else {
		t.Fatalf("expected subprocess adapter in this test")
	}
	// Reset to no-op
	m.SetEventPublisher(nil)
	// Should not panic
}
