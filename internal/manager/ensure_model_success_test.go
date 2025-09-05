package manager

import (
	"context"
	"testing"
)

func TestEnsureModel_SetsCurOnSuccess(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	if err := m.EnsureModel(context.Background(), "new-model"); err != nil {
		t.Fatalf("EnsureModel: %v", err)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cur == nil || m.cur.ID != "new-model" || m.state != StateReady {
		t.Fatalf("expected cur=new-model and state=ready; got cur=%+v state=%s", m.cur, m.state)
	}
}
