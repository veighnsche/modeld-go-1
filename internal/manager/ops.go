package manager

import "context"

// Switch kicks off an async model switch/ensure and returns an operation ID.
// The operation runs in the background; callers can poll Status() to observe
// state transitions. This is a minimal implementation sufficient for tests.
func (m *Manager) Switch(ctx context.Context, modelID string) (string, error) {
    op := m.nextOpID()
    go func(opID string) {
        // Use a detached context so background work isn't canceled when the
        // caller context is canceled; we still respect shutdown via EnsureInstance.
        _ = m.EnsureInstance(context.Background(), modelID)
        // In a fuller implementation we would emit events to subscribers here.
    }(op)
    return op, nil
}
