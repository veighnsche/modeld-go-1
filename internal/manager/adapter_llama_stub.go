//go:build !llama

package manager

// This file provides a no-CGO stub for the llama adapter. It is compiled when
// the 'llama' build tag is NOT set, keeping default builds and CI CGO-free.
// The real adapter lives in adapter_llamacpp_llama.go (tagged 'llama').

import (
	"context"
)

// llamaAdapter is a stub that satisfies InferenceAdapter but refuses to run
// inference without the 'llama' build tag available. This avoids any mocked
// behavior in production binaries built without CGO support.

type llamaAdapter struct {
	ctxSize int
	threads int
}

func NewLlamaAdapter(ctxSize, threads int) InferenceAdapter {
	return &llamaAdapter{ctxSize: ctxSize, threads: threads}
}

type llamaSession struct {
	// No real resources in the stub.
}

func (a *llamaAdapter) Start(modelPath string, params InferParams) (InferSession, error) {
	// Fail fast: llama runtime not available in this build.
	return nil, ErrDependencyUnavailable("llama support not built (missing 'llama' build tag)")
}

func (s *llamaSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	// Should never be called because Start returns an error, but return a clear error anyway.
	select {
	case <-ctx.Done():
		return FinalResult{}, ctx.Err()
	default:
	}
	return FinalResult{}, ErrDependencyUnavailable("llama support not built (missing 'llama' build tag)")
}

func (s *llamaSession) Close() error {
	// Nothing to free in the stub.
	return nil
}

