package manager

import "testing"

func TestNextOpIDIncrementsAndFormats(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	id1 := m.nextOpID()
	id2 := m.nextOpID()
	if id1 == id2 || id1 == "" || id2 == "" {
		t.Fatalf("expected distinct non-empty op IDs: %q vs %q", id1, id2)
	}
	if id1[:3] != "op-" || id2[:3] != "op-" {
		t.Fatalf("expected op- prefix: %q %q", id1, id2)
	}
}

func TestSetEventPublisherNilResetsNoop(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	m.SetEventPublisher(nil)
	// no panic and no observable state; just ensure the call path works
}
