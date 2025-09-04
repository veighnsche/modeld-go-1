package manager

import (
	"time"

	"modeld/pkg/types"
)

// Defaults applied when corresponding ManagerConfig fields are unset.
const (
	defaultMaxQueueDepth = 32
	defaultMaxWait       = 30 * time.Second
)

// ManagerConfig encapsulates all tunables for Manager construction.
type ManagerConfig struct {
	Registry     []types.Model
	BudgetMB     int
	MarginMB     int
	DefaultModel string
	MaxQueueDepth int
	MaxWait       time.Duration
}

// NewWithConfig constructs a Manager from ManagerConfig.
func NewWithConfig(cfg ManagerConfig) *Manager {
	m := &Manager{
		state:        StateLoading,
		registry:     cfg.Registry,
		budgetMB:     cfg.BudgetMB,
		marginMB:     cfg.MarginMB,
		defaultModel: cfg.DefaultModel,
		instances:    make(map[string]*Instance),
	}
	// Apply defaults if unset
	if cfg.MaxQueueDepth <= 0 {
		m.maxQueueDepth = defaultMaxQueueDepth
	} else {
		m.maxQueueDepth = cfg.MaxQueueDepth
	}
	if cfg.MaxWait <= 0 {
		m.maxWait = defaultMaxWait
	} else {
		m.maxWait = cfg.MaxWait
	}
	return m
}
