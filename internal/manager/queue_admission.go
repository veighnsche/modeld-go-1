package manager

import (
	"context"
	"time"
)

// beginGeneration reserves a queue slot and then the single in-flight slot.
// Returns a release func to be deferred.
func (m *Manager) beginGeneration(ctx context.Context, modelID string) (func(), error) {
	m.mu.RLock()
	inst := m.instances[modelID]
	m.mu.RUnlock()
	if inst == nil {
		return func() {}, modelNotFoundError{id: modelID}
	}
	// If draining, reject new work to allow graceful shutdown/unload
	if inst.State == StateDraining {
		return func() {}, tooBusyError{modelID: modelID}
	}

	// Fast path: respect an already-canceled context
	if err := ctx.Err(); err != nil {
		return func() {}, err
	}

	// Try to reserve a queue slot with timeout (pooled timer to reduce allocations)
	timer := time.NewTimer(m.maxWait)
	defer timer.Stop()
	select {
	case inst.queueCh <- struct{}{}:
		// reserved queue slot
	case <-ctx.Done():
		return func() {}, ctx.Err()
	case <-timer.C:
		return func() {}, tooBusyError{modelID: modelID}
	}

	// Wait to acquire the single in-flight slot
	acquired := false
	defer func() {
		if !acquired {
			<-inst.queueCh
		}
	}()
	// Check for cancellation again before blocking on gen slot
	if err := ctx.Err(); err != nil {
		return func() {}, err
	}
	timer2 := time.NewTimer(m.maxWait)
	defer timer2.Stop()
	select {
	case inst.genCh <- struct{}{}:
		acquired = true
		// update last used
		m.mu.Lock()
		inst.LastUsed = time.Now()
		m.mu.Unlock()
		return func() { <-inst.genCh; <-inst.queueCh }, nil
	case <-ctx.Done():
		return func() {}, ctx.Err()
	case <-timer2.C:
		return func() {}, tooBusyError{modelID: modelID}
	}
}
