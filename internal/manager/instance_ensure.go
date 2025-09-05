package manager

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"time"
	"log"
)

// EnsureInstance ensures a model instance is initialized and marked ready
// according to current resource budgeting and readiness state.
func (m *Manager) EnsureInstance(ctx context.Context, modelID string) error {
	startTs := time.Now()
	if modelID == "" {
		// If unspecified, use default if present; else no-op for now
		modelID = m.defaultModel
		if modelID == "" {
			return nil
		}
	}
	log.Printf("manager event=ensure_start model=%q", modelID)
	m.publisher.Publish(Event{Name: "ensure_start", ModelID: modelID, Fields: map[string]any{}})

	m.mu.RLock()
	inst, ok := m.instances[modelID]
	ready := ok && inst != nil && inst.State == StateReady
	m.mu.RUnlock()
	if ready {
		// Upgrade to write lock to safely mutate LastUsed and re-check state
		m.mu.Lock()
		if inst2, ok2 := m.instances[modelID]; ok2 && inst2 != nil && inst2.State == StateReady {
			inst2.LastUsed = time.Now()
			m.mu.Unlock()
			return nil
		}
		m.mu.Unlock()
		// If state changed in between, continue with ensure path
	}

	// Resolve model from registry
	mdl, ok := m.getModelByID(modelID)
	if !ok {
		log.Printf("manager event=ensure_model_not_found model=%q", modelID)
		m.publisher.Publish(Event{Name: "ensure_model_not_found", ModelID: modelID, Fields: map[string]any{}})
		return ErrModelNotFound(modelID)
	}
	reqMB := m.estimateVRAMMB(mdl)

	// Evict until it fits budget + margin, if budget configured
	if m.budgetMB > 0 {
		if err := m.evictUntilFits(reqMB); err != nil {
			log.Printf("manager event=ensure_budget_fail model=%q err=%v", modelID, err)
			m.publisher.Publish(Event{Name: "ensure_budget_fail", ModelID: modelID, Fields: map[string]any{"error": err.Error()}})
			return err
		}
	}

	// Perform per-instance load/warmup state transition
	m.mu.Lock()
	m.state = StateLoading
	m.err = ""
	// Create loading instance if not present
	if m.instances == nil {
		m.instances = make(map[string]*Instance)
	}
	inst, existed := m.instances[modelID]
	addedNow := false
	if !existed || inst == nil {
		inst = &Instance{
			ID:        modelID,
			State:     StateLoading,
			LastUsed:  time.Now(),
			EstVRAMMB: reqMB,
			genCh:     make(chan struct{}, 1),
			queueCh:   make(chan struct{}, m.maxQueueDepth),
		}
		m.instances[modelID] = inst
		addedNow = true
	} else {
		inst.State = StateLoading
		inst.EstVRAMMB = reqMB
		inst.LastUsed = time.Now()
	}
	m.mu.Unlock()

	// If using subprocess adapter, proactively spawn the runtime so readiness transitions reflect real state.
	if sa, ok := m.adapter.(*llamaSubprocessAdapter); ok {
		if _, err := sa.ensureProcess(mdl.Path); err != nil {
			m.mu.Lock()
			m.state = StateError
			m.err = err.Error()
			m.mu.Unlock()
			log.Printf("manager event=ensure_spawn_error model=%q err=%v", modelID, err)
			m.publisher.Publish(Event{Name: "ensure_spawn_error", ModelID: modelID, Fields: map[string]any{"error": err.Error()}})
			return err
		}
		// Record port and PID on instance for status visibility
		if pid, base, _, ok2 := sa.getProcInfo(mdl.Path); ok2 {
			if u, err := url.Parse(base); err == nil {
				if _, portStr, err2 := net.SplitHostPort(u.Host); err2 == nil {
					if portNum, e := strconv.Atoi(portStr); e == nil {
						m.mu.Lock()
						inst.Port = portNum
						inst.PID = pid
						m.mu.Unlock()
					}
				}
			}
		}
		log.Printf("manager event=ensure_spawn_ready model=%q pid=%d port=%d", modelID, inst.PID, inst.Port)
		m.publisher.Publish(Event{Name: "ensure_spawn_ready", ModelID: modelID, Fields: map[string]any{"pid": inst.PID, "port": inst.Port}})
	}

	// Warmup sleep to preserve readiness transitions.
	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		m.mu.Lock()
		m.state = StateError
		m.err = ctx.Err().Error()
		m.mu.Unlock()
		return ctx.Err()
	}

	// Commit instance as ready after warmup
	m.mu.Lock()
	if addedNow {
		// Only add to used estimate when we actually added a new instance
		m.usedEstMB += reqMB
	}
	inst.State = StateReady
	inst.LastUsed = time.Now()
	m.cur = &ModelInfo{ID: modelID}
	m.state = StateReady
	m.err = ""
	m.mu.Unlock()
	log.Printf("manager event=ensure_ready model=%q dur_ms=%d", modelID, time.Since(startTs)/time.Millisecond)
	m.publisher.Publish(Event{Name: "ensure_ready", ModelID: modelID, Fields: map[string]any{"dur_ms": int(time.Since(startTs)/time.Millisecond)}})
	return nil
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
