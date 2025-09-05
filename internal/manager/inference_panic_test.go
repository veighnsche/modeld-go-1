package manager

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"modeld/pkg/types"
)

type panicSession struct{}

func (p *panicSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	panic("boom")
}
func (p *panicSession) Close() error { return nil }

type panicAdapter struct{}

func (p *panicAdapter) Start(modelPath string, params InferParams) (InferSession, error) {
	return &panicSession{}, nil
}

// TestInferPanicRecovery verifies that Manager.Infer recovers from panics in the adapter
// and returns an error instead of crashing.
func TestInferPanicRecovery(t *testing.T) {
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m1", Path: "dummy.gguf"}}, DefaultModel: "m1"})
	m.SetInferenceAdapter(&panicAdapter{})
	var buf bytes.Buffer
	err := m.Infer(context.Background(), types.InferRequest{Prompt: "hi"}, &buf, func() {})
	if err == nil {
		t.Fatalf("expected error due to panic, got nil")
	}
	if !errors.Is(err, context.Canceled) { // not strictly required; just ensure it's an error
		// ok: any non-nil error is fine
	}
}
