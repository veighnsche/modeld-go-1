package manager

import (
	"os"

	"modeld/pkg/types"
)

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
        // If we cannot stat the file, return a conservative minimum of 1MB
        // to avoid bypassing budget checks due to an unknown size.
        return 1
    }
    mb := int(fi.Size() / (1024 * 1024))
    if mb <= 0 {
        mb = 1
    }
    return mb
}
