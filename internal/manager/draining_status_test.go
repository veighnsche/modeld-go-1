package manager

import (
	"testing"
	"time"
)

func TestStatusCountsWarmupAndDraining(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	m.mu.Lock()
	m.instances["a"] = &Instance{ID: "a", State: StateLoading, LastUsed: time.Now(), EstVRAMMB: 10, genCh: make(chan struct{}, 1), queueCh: make(chan struct{}, 2)}
	m.instances["b"] = &Instance{ID: "b", State: StateDraining, LastUsed: time.Now(), EstVRAMMB: 20, genCh: make(chan struct{}, 1), queueCh: make(chan struct{}, 2)}
	m.mu.Unlock()
	st := m.Status()
	if st.WarmupsInProgress != 1 {
		t.Fatalf("expected WarmupsInProgress=1, got %d", st.WarmupsInProgress)
	}
	if st.DrainingCount != 1 {
		t.Fatalf("expected DrainingCount=1, got %d", st.DrainingCount)
	}
}
