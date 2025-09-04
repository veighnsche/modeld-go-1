package manager

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"modeld/pkg/types"
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

// tooBusyError signals queue timeout/overflow for 429 mapping.
type tooBusyError struct{ modelID string }
func (e tooBusyError) Error() string { return "too busy: " + e.modelID }

// IsTooBusy reports whether err indicates backpressure (return 429).
func IsTooBusy(err error) bool {
    _, ok := err.(tooBusyError)
    return ok
}

// beginGeneration reserves a queue slot and then the single in-flight slot.
// Returns a release func to be deferred.
func (m *Manager) beginGeneration(ctx context.Context, modelID string) (func(), error) {
    m.mu.RLock()
    inst := m.instances[modelID]
    m.mu.RUnlock()
    if inst == nil { return func(){}, modelNotFoundError{id: modelID} }

    // Try to reserve a queue slot with timeout
    select {
    case inst.queueCh <- struct{}{}:
        // reserved queue slot
    case <-ctx.Done():
        return func(){}, ctx.Err()
    case <-time.After(m.maxWait):
        return func(){}, tooBusyError{modelID: modelID}
    }

    // Wait to acquire the single in-flight slot
    acquired := false
    defer func(){ if !acquired { <-inst.queueCh } }()
    select {
    case inst.genCh <- struct{}{}:
        acquired = true
        // update last used
        m.mu.Lock(); inst.LastUsed = time.Now(); m.mu.Unlock()
        return func(){ <-inst.genCh; <-inst.queueCh }, nil
    case <-ctx.Done():
        return func(){}, ctx.Err()
    case <-time.After(m.maxWait):
        return func(){}, tooBusyError{modelID: modelID}
    }
}

type Snapshot struct {
	State        State
	CurrentModel *ModelInfo
	Err          string
}

// ErrModelNotFound returns an error when a requested model id is not present in the registry.
type modelNotFoundError struct{ id string }

func (e modelNotFoundError) Error() string { return "model not found: " + e.id }

func ErrModelNotFound(id string) error { return modelNotFoundError{id: id} }

// IsModelNotFound reports whether the error indicates a missing model id.
func IsModelNotFound(err error) bool {
    _, ok := err.(modelNotFoundError)
    return ok
}

// Instance represents a live model context (one per model id).
type Instance struct {
	ID         string
	State      State
	LastUsed   time.Time
	EstVRAMMB  int
	// Queueing primitives
	genCh   chan struct{} // size 1: single in-flight generation
	queueCh chan struct{} // buffered: queue slots
	// TODO: adapter
}

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

func New(reg []types.Model, budgetMB, marginMB int, defaultModel string) *Manager {
	return &Manager{
		state:        StateLoading,
		registry:     reg,
		budgetMB:     budgetMB,
		marginMB:     marginMB,
		defaultModel: defaultModel,
		maxQueueDepth: 32,
		maxWait:       30 * time.Second,
	}
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

// ListModels returns the configured registry.
func (m *Manager) ListModels() []types.Model {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// return a shallow copy to avoid external mutation
	out := make([]types.Model, len(m.registry))
	copy(out, m.registry)
	return out
}

// EnsureInstance is a placeholder for multi-instance ensure logic.
// For now it delegates to EnsureModel to preserve behavior.
func (m *Manager) EnsureInstance(ctx context.Context, modelID string) error {
	if modelID == "" {
		// If unspecified, use default if present; else no-op for now
		modelID = m.defaultModel
		if modelID == "" {
			return nil
		}
	}

	m.mu.RLock()
	if inst, ok := m.instances[modelID]; ok && inst.State == StateReady {
		inst.LastUsed = time.Now()
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	// Resolve model from registry
	mdl, ok := m.getModelByID(modelID)
	if !ok {
		return ErrModelNotFound(modelID)
	}
	reqMB := m.estimateVRAMMB(mdl)

	// Evict until it fits budget + margin, if budget configured
	if m.budgetMB > 0 {
		if err := m.evictUntilFits(reqMB); err != nil {
			return err
		}
	}

	// Simulate load/warmup
	m.mu.Lock()
	m.state = StateLoading
	m.err = ""
	m.mu.Unlock()

	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		m.mu.Lock()
		m.state = StateError
		m.err = ctx.Err().Error()
		m.mu.Unlock()
		return ctx.Err()
	}

	// Commit instance
	m.mu.Lock()
	if m.instances == nil { m.instances = make(map[string]*Instance) }
	inst := &Instance{
		ID:        modelID,
		State:     StateReady,
		LastUsed:  time.Now(),
		EstVRAMMB: reqMB,
		genCh:     make(chan struct{}, 1),
		queueCh:   make(chan struct{}, m.maxQueueDepth),
	}
	m.instances[modelID] = inst
	m.usedEstMB += reqMB
	m.cur = &ModelInfo{ID: modelID}
	m.state = StateReady
	m.err = ""
	m.mu.Unlock()
	return nil
}

// Infer centralizes inference behavior. For MVP it ensures the instance
// and writes placeholder NDJSON chunks to the provided writer.
func (m *Manager) Infer(ctx context.Context, req types.InferRequest, w io.Writer, flusher func()) error {
    // Resolve target model id
    modelID := req.Model
    if modelID == "" {
        modelID = m.defaultModel
    }
    if err := m.EnsureInstance(ctx, modelID); err != nil {
        return err
    }
    // Admission: per-instance FIFO queue, single in-flight
    release, err := m.beginGeneration(ctx, modelID)
    if err != nil { return err }
    defer release()
	chunks := []string{"{\"token\":\"Hello\"}", "{\"token\":\",\"}", "{\"token\":\" world\"}", "{\"done\":true}"}
	for i, ch := range chunks {
		if _, err := io.WriteString(w, ch+"\n"); err != nil {
			return err
		}
		if flusher != nil {
			flusher()
		}
		if i < len(chunks)-1 {
			select {
			case <-time.After(10 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

// Status builds a detailed status response for /status.
func (m *Manager) Status() types.StatusResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()
	resp := types.StatusResponse{
		BudgetMB: m.budgetMB,
		UsedMB:   m.usedEstMB,
		MarginMB: m.marginMB,
		Error:    m.err,
	}
	resp.Instances = make([]types.InstanceStatus, 0, len(m.instances))
	for _, inst := range m.instances {
		resp.Instances = append(resp.Instances, types.InstanceStatus{
			ModelID:   inst.ID,
			State:     string(inst.State),
			LastUsed:  inst.LastUsed.Unix(),
			EstVRAMMB: inst.EstVRAMMB,
			QueueLen:  0,
		})
	}
	return resp
}

// Helper: find model in registry by id.
func (m *Manager) getModelByID(id string) (types.Model, bool) {
	for _, mdl := range m.registry {
		if mdl.ID == id {
			return mdl, true
		}
	}
	return types.Model{}, false
}

// Helper: estimate VRAM based on file size (MB). Returns 0 on error.
func (m *Manager) estimateVRAMMB(mdl types.Model) int {
	fi, err := os.Stat(mdl.Path)
	if err != nil {
		return 0
	}
	mb := int(fi.Size() / (1024 * 1024))
	if mb <= 0 {
		mb = 1
	}
	return mb
}

// Evict LRU idle instances until required MB fits budget + margin.
func (m *Manager) evictUntilFits(requiredMB int) error {
	deadline := time.Now().Add(1 * time.Second)
	for {
		m.mu.Lock()
		fits := (m.usedEstMB + requiredMB + m.marginMB) <= m.budgetMB
		if fits {
			m.mu.Unlock()
			return nil
		}
		// Pick LRU instance (oldest LastUsed)
		var lru *Instance
		for _, inst := range m.instances {
			if lru == nil || inst.LastUsed.Before(lru.LastUsed) {
				lru = inst
			}
		}
		if lru == nil {
			// nothing to evict
			m.mu.Unlock()
			return nil
		}
		// Evict it
		delete(m.instances, lru.ID)
		m.usedEstMB -= lru.EstVRAMMB
		m.mu.Unlock()

		if time.Now().After(deadline) {
			return nil
		}
		// loop to re-check
	}
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
