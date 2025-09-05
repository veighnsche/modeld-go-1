//go:build integration
// +build integration

package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSubprocessSession_GenerateStreamsTokens(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		enc := func(s string) string {
			msg := struct {
				Object  string `json:"object"`
				Choices []struct {
					Delta struct{ Content string `json:"content"` } `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}{Object: "chat.completion.chunk"}
			msg.Choices = make([]struct {
				Delta        struct{ Content string `json:"content"` } `json:"delta"`
				FinishReason string `json:"finish_reason"`
			}, 1)
			msg.Choices[0].Delta.Content = s
			b, _ := json.Marshal(msg)
			return "data: " + string(b) + "\n"
		}
		_, _ = w.Write([]byte(enc("A")))
		if f, ok := w.(http.Flusher); ok { f.Flush() }
		time.Sleep(5 * time.Millisecond)
		_, _ = w.Write([]byte(enc("B")))
		if f, ok := w.(http.Flusher); ok { f.Flush() }
		time.Sleep(5 * time.Millisecond)
		_, _ = w.Write([]byte("data: [DONE]\n"))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := &llamaSubprocessAdapter{httpClient: &http.Client{Timeout: 0}}
	sess := &llamaSubprocessSession{a: a, baseURL: ts.URL, params: InferParams{MaxTokens: 8}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var s string
	_, err := sess.Generate(ctx, "hello", func(tok string) error { s += tok; return nil })
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if s != "AB" {
		t.Fatalf("unexpected content: %q", s)
	}
}
