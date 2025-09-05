package manager

import (
	"context"
	"io"
	"time"

	"modeld/pkg/types"
)

// Infer centralizes inference behavior. For MVP it ensures the instance
// and writes placeholder NDJSON chunks to the provided writer.
func (m *Manager) Infer(ctx context.Context, req types.InferRequest, w io.Writer, flusher func()) error {
	// Resolve target model id
	modelID := req.Model
	if modelID == "" {
		modelID = m.defaultModel
		if modelID == "" {
			// No model specified and no default configured
			return modelNotFoundError{id: "(unspecified)"}
		}
	}
	if err := m.EnsureInstance(ctx, modelID); err != nil {
		return err
	}
	// Admission: per-instance FIFO queue, single in-flight
	release, err := m.beginGeneration(ctx, modelID)
	if err != nil {
		return err
	}
	defer release()
	chunks := []string{"{\"token\":\"Hello\"}", "{\"token\":\",\"}", "{\"token\":\" world\"}", "{\"done\":true}"}
	for i, ch := range chunks {
		if _, err := io.WriteString(w, ch+"\n"); err != nil {
			return err
		}
		if flusher != nil {
			flusher()
		}
		if i < len(chunks)-1 {
			select {
			case <-time.After(10 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}
