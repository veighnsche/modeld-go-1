//go:build !llama_server

package manager

// This stub disables the external llama-server runtime when the 'llama_server'
// build tag is not set. The in-process go-llama.cpp adapter remains available
// behind the 'llama' build tag.

// stopInstanceRuntime is a no-op without the llama_server runtime.
func (m *Manager) stopInstanceRuntime(inst *Instance) {}
