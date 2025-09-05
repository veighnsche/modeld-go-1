package manager

import (
	"os"
)

// SanityReport describes runtime checks for external dependencies.
type SanityReport struct {
	RealInferEnabled bool   `json:"real_infer_enabled"`
	LlamaFound       bool   `json:"llama_found"`
	LlamaPath        string `json:"llama_path,omitempty"`
	Error            string `json:"error,omitempty"`
}

// SanityCheck validates that required dependencies are available for inference.
// It does not mutate state and is safe to call at any time.
func (m *Manager) SanityCheck() SanityReport {
	r := SanityReport{RealInferEnabled: m.RealInferEnabled}
	if !m.RealInferEnabled {
		return r
	}
	// In-process inference: adapter must be initialized. We don't require external binaries.
	if m.adapter == nil {
		r.LlamaFound = false
		r.Error = "llama adapter not initialized"
		return r
	}
	// Optionally perform a lightweight check: verify default model path exists if set.
	r.LlamaFound = true
	if m.defaultModel != "" {
		if mdl, ok := m.getModelByID(m.defaultModel); ok && mdl.Path != "" {
			if fi, err := os.Stat(mdl.Path); err == nil && !fi.IsDir() {
				r.LlamaPath = mdl.Path
			}
		}
	}
	return r
}
