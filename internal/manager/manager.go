package manager

import (
"context"
"sync"
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
mu     sync.RWMutex
state  State
cur    *ModelInfo
err    string
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
return "op-"+modelID, nil
}

func (m *Manager) Ready() bool {
m.mu.RLock(); defer m.mu.RUnlock()
return m.state == StateReady && m.cur != nil
}
