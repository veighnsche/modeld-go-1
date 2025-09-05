package manager

import (
	"context"
	"encoding/json"
	"io"
	"strings"
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

	// Feature flag: enable real inference when configured and an adapter is present.
	if m.RealInferEnabled && m.adapter != nil {
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
			if _, e := io.WriteString(w, tokenLine(tok)); e != nil {
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

	// Fallback placeholder (legacy behavior)
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

// isTruthy interprets common truthy values.
func isTruthy(v string) bool {
	if v == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// tokenLine formats a token NDJSON line.
func tokenLine(tok string) string {
	// naive JSON escaping for quotes and backslashes; sufficient for tokens
	esc := strings.ReplaceAll(tok, "\\", "\\\\")
	esc = strings.ReplaceAll(esc, "\"", "\\\"")
	return "{\"token\":\"" + esc + "\"}\n"
}
