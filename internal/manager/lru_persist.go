package manager

import (
	"encoding/json"
	"os"
)

type lruRecord struct {
	LastUsedUnix int64 `json:"last_used_unix"`
	EstVRAMMB    int   `json:"est_vram_mb"`
}

func (m *Manager) loadLRUMetadata() {
	if m.lruPath == "" {
		return
	}
	f, err := os.Open(m.lruPath)
	if err != nil {
		return
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var data map[string]lruRecord
	if err := dec.Decode(&data); err == nil {
		m.lruMeta = data
	}
}

func (m *Manager) saveLRUMetadata() {
	if m.lruPath == "" {
		return
	}
	// Snapshot under lock
	m.mu.RLock()
	snap := make(map[string]lruRecord, len(m.instances))
	for id, inst := range m.instances {
		snap[id] = lruRecord{LastUsedUnix: inst.LastUsed.Unix(), EstVRAMMB: inst.EstVRAMMB}
	}
	m.mu.RUnlock()
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.lruPath, b, 0o644)
}
