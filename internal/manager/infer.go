package manager

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	// Feature flag: enable real inference when configured.
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

		// Otherwise, call llama-server over HTTP.
		m.mu.RLock()
		inst := m.instances[modelID]
		port := 0
		if inst != nil {
			port = inst.Port
		}
		m.mu.RUnlock()
		if port == 0 {
			return fmt.Errorf("instance for %s has no runtime port", modelID)
		}
		// Build request body for llama.cpp /completion streaming endpoint
		body := map[string]any{
			"prompt":      req.Prompt,
			"stream":      true,
			"n_predict":   req.MaxTokens,
			"temperature": req.Temperature,
			"top_p":       req.TopP,
		}
		if len(req.Stop) > 0 {
			body["stop"] = req.Stop
		}
		if req.Seed != 0 {
			body["seed"] = req.Seed
		}
		jb, _ := json.Marshal(body)
		url := fmt.Sprintf("http://127.0.0.1:%d/completion", port)
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jb))
		httpReq.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("llama-server status %d: %s", resp.StatusCode, string(b))
		}
		// Stream lines; handle both raw NDJSON and SSE 'data: ' prefixed lines.
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
		var contentBuilder strings.Builder
		finishReason := "stop"
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "data:") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
			if line == "[DONE]" {
				break
			}
			var chunk map[string]any
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				// If not JSON, skip silently
				continue
			}
			// Try multiple fields where token may appear
			tok := ""
			if v, ok := chunk["token"].(string); ok {
				tok = v
			} else if v, ok := chunk["content"].(string); ok {
				tok = v
			} else if v, ok := chunk["completion"].(string); ok {
				// Some servers send cumulative completion; diffing is complex. Emit as-is.
				tok = v
			}
			if tok != "" {
				contentBuilder.WriteString(tok)
				if _, err := io.WriteString(w, tokenLine(tok)); err != nil {
					return err
				}
				if flusher != nil {
					flusher()
				}
			}
			// Detect stop conditions
			if v, ok := chunk["stop"].(bool); ok && v {
				if fr, ok2 := chunk["finish_reason"].(string); ok2 && fr != "" {
					finishReason = fr
				}
				break
			}
		}
		// Emit final line
		end := map[string]any{
			"done":          true,
			"content":       contentBuilder.String(),
			"finish_reason": finishReason,
			"usage": map[string]int{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
			},
		}
		endb, _ := json.Marshal(end)
		if _, err := w.Write(append(endb, '\n')); err != nil {
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
