package manager

import (
	"bytes"
	"context"
	"testing"

	"modeld/pkg/types"
)

type nopSession struct{}

func (n *nopSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	// No tokens; just final content
	return FinalResult{Content: "ok"}, nil
}
func (n *nopSession) Close() error { return nil }

type nopAdapter struct{}

func (n *nopAdapter) Start(modelPath string, params InferParams) (InferSession, error) {
	return &nopSession{}, nil
}

func TestInfer_FlusherPanicIsRecovered(t *testing.T) {
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: "m.gguf"}}, DefaultModel: "m"})
	m.SetInferenceAdapter(&nopAdapter{})
	var buf bytes.Buffer
	panicFlusher := func() { panic("boom") }
	err := m.Infer(context.Background(), types.InferRequest{Prompt: "hi"}, &buf, panicFlusher)
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}
	if s := buf.String(); s == "" {
		t.Fatalf("expected some NDJSON output, got empty")
	}
}
