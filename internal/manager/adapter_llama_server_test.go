//go:build integration
// +build integration

package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// sseWriter helps write SSE-style lines.
type sseWriter struct{ w http.ResponseWriter }

func (sw sseWriter) writeLine(line string) {
	sw.w.Write([]byte(line))
	sw.w.Write([]byte("\n"))
	if f, ok := sw.w.(http.Flusher); ok {
		f.Flush()
	}
}

func TestLlamaServerAdapter_OpenAIStream_Basic(t *testing.T) {
	// Mock server emitting OpenAI-style SSE stream for /v1/completions
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		sw := sseWriter{w: w}
		// Emit two fragments and then [DONE]
		frag := func(s string) string {
			// Build minimal OpenAI streaming JSON
			msg := struct {
				Object  string `json:"object"`
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}{Object: "chat.completion.chunk"}
			msg.Choices = make([]struct {
				Delta        struct{ Content string `json:"content"` } `json:"delta"`
				FinishReason string `json:"finish_reason"`
			}, 1)
			msg.Choices[0].Delta.Content = s
			b, _ := json.Marshal(msg)
			return "data: " + string(b)
		}
		sw.writeLine(frag("Hello"))
		time.Sleep(5 * time.Millisecond)
		sw.writeLine(frag(" World"))
		time.Sleep(5 * time.Millisecond)
		sw.writeLine("data: [DONE]")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	adapter := NewLlamaServerAdapter(ts.URL, "", true, 5*time.Second, 2*time.Second)
	sess, err := adapter.Start("test-model", InferParams{MaxTokens: 16})
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer sess.Close()

	var b strings.Builder
	onTok := func(tok string) error {
		b.WriteString(tok)
		return nil
	}
	_, err = sess.Generate(testCtx(t), "Say hi", onTok)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if got := b.String(); got != "Hello World" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestLlamaServerAdapter_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "boom"}})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	adapter := NewLlamaServerAdapter(ts.URL, "", true, 3*time.Second, 1*time.Second)
	sess, err := adapter.Start("m", InferParams{MaxTokens: 8})
	if err != nil { t.Fatalf("start: %v", err) }
	defer sess.Close()

	_, err = sess.Generate(testCtx(t), "hello", func(string) error { return nil })
	if err == nil {
		t.Fatalf("expected error on HTTP 500")
	}
}

func TestLlamaServerAdapter_ContextCancel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// stream very slowly so timeout/cancel triggers
		for i := 0; i < 5; i++ {
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n"))
			if f, ok := w.(http.Flusher); ok { f.Flush() }
			time.Sleep(200 * time.Millisecond)
		}
		_, _ = w.Write([]byte("data: [DONE]\n"))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	adapter := NewLlamaServerAdapter(ts.URL, "", true, 250*time.Millisecond, 1*time.Second)
	sess, err := adapter.Start("m", InferParams{MaxTokens: 8})
	if err != nil { t.Fatalf("start: %v", err) }
	defer sess.Close()

	_, err = sess.Generate(context.Background(), "hello", func(string) error { return nil })
	if err == nil {
		t.Fatalf("expected context deadline exceeded or cancel error due to short req timeout")
	}
}
