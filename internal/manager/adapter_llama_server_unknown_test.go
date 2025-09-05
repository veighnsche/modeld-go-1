//go:build integration
// +build integration

package manager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLlamaServerAdapter_UnknownStreamLine(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// send a non-JSON data line to exercise unknown_stream_line path
		_, _ = w.Write([]byte("data: this-is-not-json\n"))
		// finish properly
		_, _ = w.Write([]byte("data: [DONE]\n"))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := NewLlamaServerAdapter(ts.URL, "", true, 2*time.Second, 1*time.Second)
	sess, err := a.Start("m", InferParams{})
	if err != nil { t.Fatalf("start: %v", err) }
	defer sess.Close()
	_, err = sess.Generate(context.Background(), "hi", func(string) error { return nil })
	if err != nil {
		// Even if unknown line is encountered, the stream should complete without hard error
		// Accept context errors but not generic errors here
		t.Fatalf("unexpected error: %v", err)
	}
}
