package manager

import (
	"sync"
	"time"

	"modeld/pkg/types"
)

type Manager struct {
	mu           sync.RWMutex
	state        State
	cur          *ModelInfo
	err          string
	registry     []types.Model
	budgetMB     int
	marginMB     int
	defaultModel string
	// Multi-instance fields
	instances    map[string]*Instance
	usedEstMB    int
	// TODO: subscribers, op ids, queue

	// Queue config
	maxQueueDepth int
	maxWait       time.Duration
}

func New(reg []types.Model, budgetMB, marginMB int, defaultModel string) *Manager {
    // Delegate to NewWithConfig to centralize defaults and option parsing
    return NewWithConfig(ManagerConfig{
        Registry:      reg,
        BudgetMB:      budgetMB,
        MarginMB:      marginMB,
        DefaultModel:  defaultModel,
        MaxQueueDepth: 0,            // use package defaults
        MaxWait:       0,            // use package defaults
    })
}

func (m *Manager) Ready() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == StateError {
		return false
	}
	// Ready if any instance is ready
	for _, inst := range m.instances {
		if inst.State == StateReady {
			return true
		}
	}
	// Fallback to legacy notion
	return m.state == StateReady && m.cur != nil
}

func (m *Manager) ListModels() []types.Model {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// return a shallow copy to avoid external mutation
	out := make([]types.Model, len(m.registry))
	copy(out, m.registry)
	return out
}
