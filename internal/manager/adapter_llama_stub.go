//go:build !llama

package manager

import (
	"context"
	"errors"
)

// llamaBuilt indicates this binary was compiled without real llama support.
var llamaBuilt = false

// NewLlamaAdapter returns a stub adapter when built without the 'llama' tag.
func NewLlamaAdapter(ctxSize, threads int) InferenceAdapter { return &llamaStub{} }

type llamaStub struct{}

type llamaStubSession struct{}

func (a *llamaStub) Start(modelPath string, params InferParams) (InferSession, error) {
	return &llamaStubSession{}, nil
}

func (s *llamaStubSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	return FinalResult{}, errors.New("llama support not built; recompile with -tags=llama")
}

func (s *llamaStubSession) Close() error { return nil }
