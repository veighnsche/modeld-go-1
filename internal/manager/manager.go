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

	// Queue config (MVP defaults)
	maxQueueDepth int
	maxWait       time.Duration
}

const (
	defaultMaxQueueDepth = 32
	defaultMaxWait       = 30 * time.Second
)

func New(reg []types.Model, budgetMB, marginMB int, defaultModel string) *Manager {
	return &Manager{
		state:         StateLoading,
		registry:      reg,
		budgetMB:      budgetMB,
		marginMB:      marginMB,
		defaultModel:  defaultModel,
		instances:     make(map[string]*Instance),
		maxQueueDepth: defaultMaxQueueDepth,
		maxWait:       defaultMaxWait,
	}
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
