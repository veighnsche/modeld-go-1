//go:build !llama

package manager

// This file provides a no-CGO stub for the llama adapter. It is compiled when
// the 'llama' build tag is NOT set, keeping default builds and CI CGO-free.
// The real adapter lives in adapter_llamacpp_llama.go (tagged 'llama').

import (
	"context"
	"errors"
	"strings"
)

// llamaAdapter is a placeholder implementation that satisfies InferenceAdapter.
// It establishes the dependency on go-llama.cpp while keeping the system
// runnable until we wire the concrete calls. Replace with a real implementation
// that loads the model and generates tokens via go-llama.cpp APIs.

type llamaAdapter struct {
	ctxSize int
	threads int
}

func NewLlamaAdapter(ctxSize, threads int) InferenceAdapter {
	return &llamaAdapter{ctxSize: ctxSize, threads: threads}
}

type llamaSession struct {
	// In a real implementation, hold references to model/context/kv cache objects.
}

func (a *llamaAdapter) Start(modelPath string, params InferParams) (InferSession, error) {
	if strings.TrimSpace(modelPath) == "" {
		return nil, errors.New("model path is empty")
	}
	// TODO: Initialize go-llama.cpp model/context here using modelPath and a.ctxSize/a.threads.
	return &llamaSession{}, nil
}

func (s *llamaSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	// Stub: echo-style tokenization to keep system functional without external server.
	// Replace with streaming generation from go-llama.cpp.
	tokens := []string{prompt}
	var b strings.Builder
	for _, t := range tokens {
		select {
		case <-ctx.Done():
			return FinalResult{}, ctx.Err()
		default:
		}
		if err := onToken(t); err != nil {
			return FinalResult{}, err
		}
		b.WriteString(t)
	}
	return FinalResult{
		Content:      b.String(),
		Usage:        Usage{PromptTokens: 0, CompletionTokens: 0, TotalTokens: 0},
		FinishReason: "stop",
	}, nil
}

func (s *llamaSession) Close() error {
	// TODO: free resources once real objects are used.
	return nil
}
