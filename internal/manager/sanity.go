package manager

import (
	"os"
)

// SanityReport describes runtime checks for external dependencies.
type SanityReport struct {
	LlamaFound bool   `json:"llama_found"`
	LlamaPath  string `json:"llama_path,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SanityCheck validates that required dependencies are available for inference.
// It does not mutate state and is safe to call at any time.
func (m *Manager) SanityCheck() SanityReport {
	r := SanityReport{}
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

// Check represents one preflight check item.
type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// Preflight runs a series of non-destructive checks to validate that inference can run.
// It focuses on:
// - Adapter presence
// - Default model configured
// - Default model path exists and is a file
// Note: it does not attempt to load the model (avoids heavy I/O at startup).
func (m *Manager) Preflight() []Check {
	var checks []Check
	// Adapter presence
	if m.adapter == nil {
		checks = append(checks, Check{Name: "adapter_present", OK: false, Message: "llama adapter not initialized"})
	} else {
		checks = append(checks, Check{Name: "adapter_present", OK: true})
	}
	// Default model configured
	if m.defaultModel == "" {
		checks = append(checks, Check{Name: "default_model_configured", OK: false, Message: "no default model configured"})
	} else {
		checks = append(checks, Check{Name: "default_model_configured", OK: true})
		// Resolve model and check path
		mdl, ok := m.getModelByID(m.defaultModel)
		if !ok {
			checks = append(checks, Check{Name: "default_model_in_registry", OK: false, Message: "default model id not found in registry"})
		} else {
			checks = append(checks, Check{Name: "default_model_in_registry", OK: true})
			if mdl.Path == "" {
				checks = append(checks, Check{Name: "default_model_path_set", OK: false, Message: "default model path is empty"})
			} else if fi, err := os.Stat(mdl.Path); err != nil {
				checks = append(checks, Check{Name: "default_model_path_exists", OK: false, Message: err.Error()})
			} else if fi.IsDir() {
				checks = append(checks, Check{Name: "default_model_path_is_file", OK: false, Message: "path points to a directory, expected a .gguf file"})
			} else {
				checks = append(checks, Check{Name: "default_model_path_exists", OK: true})
				checks = append(checks, Check{Name: "default_model_path_is_file", OK: true})
			}
		}
	}
	return checks
}
