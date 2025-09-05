package manager

import "testing"

func TestReady_FallbackOnStateAndCur(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	m.mu.Lock()
	m.state = StateReady
	m.cur = &ModelInfo{ID: "m"}
	m.mu.Unlock()
	if !m.Ready() {
		t.Fatalf("expected Ready() to be true when state=ready and cur set")
	}
	m.mu.Lock()
	m.state = StateError
	m.mu.Unlock()
	if m.Ready() {
		t.Fatalf("expected Ready() false when state=error")
	}
}
