package manager

import "testing"

func TestSnapshotReturnsState(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	m.mu.Lock()
	m.state = StateError
	m.err = "boom"
	m.cur = &ModelInfo{ID: "m"}
	m.mu.Unlock()
	s := m.Snapshot()
	if s.State != StateError || s.Err == "" || s.CurrentModel == nil || s.CurrentModel.ID != "m" {
		t.Fatalf("unexpected snapshot: %+v", s)
	}
}
