package manager

import (
	"modeld/pkg/types"
)

// Snapshot returns a read-only view of the manager state.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Snapshot{State: m.state, CurrentModel: m.cur, Err: m.err}
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
		State:    string(m.state),
	}
	resp.Instances = make([]types.InstanceStatus, 0, len(m.instances))
	warmups := 0
	draining := 0
	for _, inst := range m.instances {
		if inst.State == StateLoading { warmups++ }
		if inst.State == StateDraining { draining++ }
		resp.Instances = append(resp.Instances, types.InstanceStatus{
			ModelID:       inst.ID,
			State:         string(inst.State),
			LastUsed:      inst.LastUsed.Unix(),
			EstVRAMMB:     inst.EstVRAMMB,
			QueueLen:      len(inst.queueCh),
			Inflight:      len(inst.genCh),
			MaxQueueDepth: cap(inst.queueCh),
			Port:          inst.Port,
			PID:           inst.PID,
		})
	}
	resp.WarmupsInProgress = warmups
	resp.DrainingCount = draining
	return resp
}
