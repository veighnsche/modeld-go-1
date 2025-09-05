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

// SanityCheck validates that required external binaries are available.
// It does not mutate state and is safe to call at any time.
func (m *Manager) SanityCheck() SanityReport {
	r := SanityReport{RealInferEnabled: m.RealInferEnabled}
	if !m.RealInferEnabled {
		return r
	}
	// Try configured path first, then discovery.
	bin := m.LlamaBin
	if bin == "" {
		bin = discoverLlamaBin()
	}
	if bin == "" {
		r.LlamaFound = false
		r.Error = "llama-server not found"
		return r
	}
	if fi, err := os.Stat(bin); err == nil && !fi.IsDir() {
		r.LlamaFound = true
		r.LlamaPath = bin
		return r
	} else {
		r.LlamaFound = false
		r.LlamaPath = bin
		if err != nil {
			r.Error = err.Error()
		} else {
			r.Error = "llama path is a directory"
		}
		return r
	}
}
