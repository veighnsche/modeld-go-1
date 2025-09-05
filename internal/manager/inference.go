package manager

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"modeld/pkg/types"
)

// Infer centralizes inference behavior. It ensures the model instance exists,
// performs inference via the configured adapter when enabled, and streams
// NDJSON token lines to the provided writer. If inference is not enabled,
// it fails fast with a dependency-unavailable error (no mocking).
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

	// Feature flag: enable inference via adapter when configured.
	if m.RealInferEnabled {
		// If an adapter is provided, prefer it.
		if m.adapter != nil {
			// Resolve model path from registry
			mdl, ok := m.getModelByID(modelID)
			if !ok || strings.TrimSpace(mdl.Path) == "" {
				return ErrModelNotFound(modelID)
			}
			// Map request parameters to adapter params (basic mapping for now)
			params := InferParams{
				Temperature:   float32(req.Temperature),
				TopP:          float32(req.TopP),
				TopK:          0, // optional, extend types.InferRequest if needed
				MaxTokens:     req.MaxTokens,
				Stop:          req.Stop,
				Seed:          int(req.Seed),
				RepeatPenalty: 0,
			}
			sess, err := m.adapter.Start(mdl.Path, params)
			if err != nil {
				return err
			}
			defer func() { _ = sess.Close() }()

			var b strings.Builder
			onTok := func(tok string) error {
				if _, e := w.Write(tokenLineJSON(tok)); e != nil {
					return e
				}
				b.WriteString(tok)
				if flusher != nil {
					flusher()
				}
				return nil
			}
			final, err := sess.Generate(ctx, req.Prompt, onTok)
			if err != nil {
				return err
			}
			// Compose final line
			content := final.Content
			if content == "" {
				content = b.String()
			}
			end := map[string]any{
				"done":          true,
				"content":       content,
				"finish_reason": final.FinishReason,
				"usage":         final.Usage,
			}
			jb, _ := json.Marshal(end)
			if _, err := w.Write(append(jb, '\n')); err != nil {
				return err
			}
			if flusher != nil {
				flusher()
			}
			return nil
		}
		// If real inference is enabled but no adapter is configured, report dependency error.
		return ErrDependencyUnavailable("llama adapter not initialized")
	}

	// Inference is not enabled; report dependency unavailable instead of mocking.
	return ErrDependencyUnavailable("inference disabled")
}

// tokenLineJSON formats a token NDJSON line using json.Marshal for correctness.
func tokenLineJSON(tok string) []byte {
	type tokenMsg struct {
		Token string `json:"token"`
	}
	b, _ := json.Marshal(tokenMsg{Token: tok})
	return append(b, '\n')
}
