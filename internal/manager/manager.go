package manager

import (
	"sync"
	"sync/atomic"
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
	// Operation sequencing (for async ops)
	opSeq        uint64
	// Subscribers (event listeners) could be added here in the future

	// Queue config
	maxQueueDepth int
	maxWait       time.Duration

	// Observability
	startTime      time.Time
	loadsTotal     uint64
	evictionsTotal uint64
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

// nextOpID returns a unique operation ID string with the prefix "op-".
func (m *Manager) nextOpID() string {
    n := atomic.AddUint64(&m.opSeq, 1)
    return "op-" + fmtUint(n)
}

// fmtUint converts a uint64 to its base-10 string representation without fmt.
func fmtUint(n uint64) string {
    if n == 0 { return "0" }
    var buf [20]byte
    i := len(buf)
    for n > 0 {
        i--
        buf[i] = byte('0' + n%10)
        n /= 10
    }
    return string(buf[i:])
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
