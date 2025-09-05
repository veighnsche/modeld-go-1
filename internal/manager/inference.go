package manager

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "strings"

    "modeld/pkg/types"
)

// Infer centralizes inference behavior. It ensures the model instance exists,
// performs inference via the configured adapter when enabled, and streams
// NDJSON token lines to the provided writer. If inference is not enabled,
// it fails fast with a dependency-unavailable error (no mocking).
func (m *Manager) Infer(ctx context.Context, req types.InferRequest, w io.Writer, flusher func()) error {
    // Convert unexpected panics into a no-op to avoid tearing down the server;
    // HTTP layer has its own recoverer for logging.
    defer func() { _ = recover() }()
    if w == nil {
        return fmt.Errorf("writer is nil")
    }
    if ctx == nil {
        return fmt.Errorf("context is nil")
    }
    // Fast-fail on canceled context
    if err := ctx.Err(); err != nil {
        return err
    }
    // Resolve target model id
    modelID := req.Model
    if modelID == "" {
        modelID = m.defaultModel
        if modelID == "" {
            // No model specified and no default configured
            return modelNotFoundError{id: "(unspecified)"}
        }
    }
    // Ensure MaxTokens is sane (>0) to avoid adapter errors; allow adapter defaults if zero
    if req.MaxTokens < 0 {
        req.MaxTokens = 0
    }
    if err := m.EnsureInstance(ctx, modelID); err != nil {
        return fmt.Errorf("ensure instance %q: %w", modelID, err)
    }
    // Admission: per-instance FIFO queue, single in-flight
    release, err := m.beginGeneration(ctx, modelID)
    if err != nil {
        return fmt.Errorf("begin generation %q: %w", modelID, err)
    }
    defer release()

    // Adapter-backed inference is the default; require an adapter.
    if m.adapter == nil {
        return ErrDependencyUnavailable("llama adapter not initialized")
    }
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
    if err := ctx.Err(); err != nil {
        return err
    }
    sess, err := m.adapter.Start(mdl.Path, params)
    if err != nil {
        return fmt.Errorf("adapter start: %w", err)
    }
    if sess == nil {
        return fmt.Errorf("adapter start returned nil session")
    }
    // Close session and retain the first close error if any (without masking an earlier error)
    var closeErr error
    defer func() {
        if e := sess.Close(); e != nil && closeErr == nil {
            closeErr = e
        }
    }()

    var b strings.Builder
    onTok := func(tok string) error {
        // Stop early if context is canceled
        if err := ctx.Err(); err != nil {
            return err
        }
        line := tokenLineJSON(tok)
        if err := writeAll(w, line); err != nil {
            return err
        }
        b.WriteString(tok)
        safeFlush(flusher)
        return nil
    }
    final, err := sess.Generate(ctx, req.Prompt, onTok)
    if err != nil {
        // Prefer context error when applicable to aid callers
        if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
            return context.Canceled
        }
        if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
            return context.DeadlineExceeded
        }
        return fmt.Errorf("adapter generate: %w", err)
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
    jb, merr := json.Marshal(end)
    if merr != nil {
        return fmt.Errorf("marshal final line: %w", merr)
    }
    if err := writeAll(w, append(jb, '\n')); err != nil {
        return err
    }
    safeFlush(flusher)
    // Return any session close error if present (and not masked by prior returns)
    return closeErr
}

// tokenLineJSON formats a token NDJSON line using json.Marshal for correctness.
func tokenLineJSON(tok string) []byte {
    type tokenMsg struct {
        Token string `json:"token"`
    }
    b, _ := json.Marshal(tokenMsg{Token: tok})
    return append(b, '\n')
}

// safeFlush invokes flusher() if non-nil and recovers from panics to avoid
// tearing down the entire request due to a misbehaving writer.
func safeFlush(flusher func()) {
    if flusher == nil {
        return
    }
    defer func() { _ = recover() }()
    flusher()
}

// writeAll writes the full buffer to w, retrying until all bytes are written or an error occurs.
func writeAll(w io.Writer, p []byte) error {
    for len(p) > 0 {
        n, err := w.Write(p)
        if err != nil {
            return err
        }
        if n <= 0 {
            return fmt.Errorf("short write: wrote %d bytes", n)
        }
        p = p[n:]
    }
    return nil
}
