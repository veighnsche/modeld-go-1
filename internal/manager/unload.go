package manager

import (
	"time"
)

// Unload initiates a graceful drain of a model instance and removes it.
// - Sets instance state to draining to reject new enqueues.
// - Waits up to drainTimeout for in-flight and queued requests to finish.
// - Stops the subprocess (spawn mode) and removes the instance entry.
func (m *Manager) Unload(modelID string) error {
	if modelID == "" {
		return ErrModelNotFound("(unspecified)")
	}
	m.mu.Lock()
	inst := m.instances[modelID]
	if inst == nil {
		m.mu.Unlock()
		return ErrModelNotFound(modelID)
	}
	inst.State = StateDraining
	m.mu.Unlock()
	m.publisher.Publish(Event{Name: "unload_start", ModelID: modelID, Fields: map[string]any{}})

	deadline := time.Now().Add(m.drainTimeout)
	for {
		m.mu.RLock()
		qlen := len(inst.queueCh)
		inflight := len(inst.genCh)
		m.mu.RUnlock()
		if inflight == 0 && qlen == 0 {
			break
		}
		if time.Now().After(deadline) {
			m.publisher.Publish(Event{Name: "unload_timeout", ModelID: modelID, Fields: map[string]any{"inflight": inflight, "queue": qlen}})
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Stop subprocess if in spawn mode
	if sa, ok := m.adapter.(*llamaSubprocessAdapter); ok {
		if mdl, ok2 := m.getModelByID(modelID); ok2 {
			_ = sa.Stop(mdl.Path)
		}
	}

	m.mu.Lock()
	// Adjust accounting and remove
	if inst2 := m.instances[modelID]; inst2 != nil {
		m.usedEstMB -= inst2.EstVRAMMB
		if m.usedEstMB < 0 {
			m.usedEstMB = 0
		}
	}
	delete(m.instances, modelID)
	if m.cur != nil && m.cur.ID == modelID {
		m.cur = nil
	}
	m.mu.Unlock()

	m.publisher.Publish(Event{Name: "unload_done", ModelID: modelID, Fields: map[string]any{}})
	return nil
}
