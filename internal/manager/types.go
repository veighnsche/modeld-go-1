package manager

import "time"

// State represents lifecycle state of the manager/instances.
type State string

const (
	StateReady   State = "ready"
	StateLoading State = "loading"
	StateError   State = "error"
)

// ModelInfo is a minimal view of the current model.
type ModelInfo struct {
	ID     string
	Name   string
	Path   string
	Quant  string
	Family string
}

// Snapshot is a read-only projection of the manager state.
type Snapshot struct {
	State        State
	CurrentModel *ModelInfo
	Err          string
}

// Instance represents a live model context (one per model id).
type Instance struct {
	ID        string
	State     State
	LastUsed  time.Time
	EstVRAMMB int
	// Queueing primitives
	genCh   chan struct{} // size 1: single in-flight generation
	queueCh chan struct{} // buffered: queue slots
	// Runtime endpoint info (when inference via external runtime is enabled)
	Port int
	// Process handle for managed runtime (e.g., llama-server)
	Proc interface{}
}
