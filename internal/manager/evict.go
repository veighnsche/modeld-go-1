package manager

import "time"

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
		// Pick LRU idle instance (no in-flight and no queued requests)
		var lru *Instance
		for _, inst := range m.instances {
			if len(inst.genCh) > 0 || len(inst.queueCh) > 0 {
				// active or has queued work; skip to avoid cancel requirement in MVP
				continue
			}
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
