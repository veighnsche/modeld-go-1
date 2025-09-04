package manager

import (
	"context"
	"sync"
	"time"
)

type State string

const (
	StateReady   State = "ready"
	StateLoading State = "loading"
	StateError   State = "error"
)

type ModelInfo struct {
	ID     string
	Name   string
	Path   string
	Quant  string
	Family string
}

type Snapshot struct {
	State        State
	CurrentModel *ModelInfo
	Err          string
}

type Manager struct {
	mu    sync.RWMutex
	state State
	cur   *ModelInfo
	err   string
	// TODO: registry, subscribers, op ids, queue
}

func New() *Manager {
	return &Manager{state: StateLoading}
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Snapshot{State: m.state, CurrentModel: m.cur, Err: m.err}
}

// Switch kicks off an async model switch (stub).
func (m *Manager) Switch(ctx context.Context, modelID string) (opID string, err error) {
	// TODO: set loading, emit events, unload->load->warmup, set ready
	return "op-" + modelID, nil
}

func (m *Manager) Ready() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == StateReady && m.cur != nil
}

// EnsureModel changes the active model if needed, else no-op.
// For MVP this is a synchronous stub that sets state transitions and
// simulates work with a small sleep. In the future, validate modelID
// against a registry and perform real unload/load/warmup.
func (m *Manager) EnsureModel(ctx context.Context, modelID string) error {
	if modelID == "" {
		return nil
	}
	m.mu.RLock()
	if m.cur != nil && m.cur.ID == modelID {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	m.state = StateLoading
	m.err = ""
	m.mu.Unlock()

	// Simulate unload/load/warmup work
	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		m.mu.Lock()
		m.state = StateError
		m.err = ctx.Err().Error()
		m.mu.Unlock()
		return ctx.Err()
	}

	m.mu.Lock()
	m.cur = &ModelInfo{ID: modelID}
	m.state = StateReady
	m.err = ""
	m.mu.Unlock()
	return nil
}
