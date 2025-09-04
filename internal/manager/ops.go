package manager

import "context"

// Switch kicks off an async model switch (stub).
func (m *Manager) Switch(ctx context.Context, modelID string) (opID string, err error) {
	// TODO: set loading, emit events, unload->load->warmup, set ready
	return "op-" + modelID, nil
}
