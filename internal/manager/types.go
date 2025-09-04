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

// Adapter represents a model runtime. This is a placeholder for future
// integration and allows the Instance to hold a reference without dictating
// a concrete implementation yet.
type Adapter interface {
    // Close releases resources associated with the adapter.
    Close() error
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
	// Adapter backing this instance (placeholder for real model runtime)
	Adapter Adapter
}
