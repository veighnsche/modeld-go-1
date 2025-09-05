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

	// Try to reserve a queue slot with timeout
	select {
	case inst.queueCh <- struct{}{}:
		// reserved queue slot
	case <-ctx.Done():
		return func() {}, ctx.Err()
	case <-time.After(m.maxWait):
		return func() {}, tooBusyError{modelID: modelID}
	}

	// Wait to acquire the single in-flight slot
	acquired := false
	defer func() {
		if !acquired {
			<-inst.queueCh
		}
	}()
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
	case <-time.After(m.maxWait):
		return func() {}, tooBusyError{modelID: modelID}
	}
}
