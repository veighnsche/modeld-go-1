//go:build llama

package manager

import (
	"context"
	"errors"
	"strings"

	llama "github.com/go-skynet/go-llama.cpp"
)

// llamaBuilt indicates this binary was compiled with real llama support.
var llamaBuilt = true

// llamaAdapter holds global config used to initialize a model instance
type llamaAdapter struct {
	ctxSize int
	threads int
}

func NewLlamaAdapter(ctxSize, threads int) InferenceAdapter {
	return &llamaAdapter{ctxSize: ctxSize, threads: threads}
}

// llamaSession owns the loaded model
type llamaSession struct {
	model      *llama.LLama
	threads    int
	baseParams InferParams
}

func (a *llamaAdapter) Start(modelPath string, params InferParams) (InferSession, error) {
	if strings.TrimSpace(modelPath) == "" {
		return nil, errors.New("model path is empty")
	}
	// Configure model options
	mo := []llama.ModelOption{
		llama.SetContext(a.ctxSize),
	}
	// Load model
	m, err := llama.New(modelPath, mo...)
	if err != nil {
		return nil, err
	}
	return &llamaSession{model: m, threads: a.threads, baseParams: params}, nil
}

func (s *llamaSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	if s.model == nil {
		return FinalResult{}, errors.New("llama model not initialized")
	}

	// Bridge token streaming to onToken and respect cancellation
	s.model.SetTokenCallback(func(tok string) bool {
		// If context canceled, signal stop
		select {
		case <-ctx.Done():
			return false
		default:
		}
		// Forward token; on error, stop generation
		if err := onToken(tok); err != nil {
			return false
		}
		return true
	})
	// Build PredictOptions from the params provided at Start
	po := mapInferParamsToPredictOptions(s.baseParams, s.threads)
	// Run prediction (blocking until done or callback returns false)
	text, err := s.model.Predict(prompt, po...)
	if err != nil {
		// Propagate context error if applicable
		if ctx.Err() != nil {
			return FinalResult{}, ctx.Err()
		}
		return FinalResult{}, err
	}
	// Basic aggregation; token counts not available without deeper hooks
	return FinalResult{
		Content:      text,
		Usage:        Usage{PromptTokens: 0, CompletionTokens: 0, TotalTokens: 0},
		FinishReason: "stop",
	}, nil
}

func (s *llamaSession) Close() error {
	if s.model != nil {
		s.model.Free()
		s.model = nil
	}
	return nil
}

// helpers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func zn(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
func zf(v, def float32) float32 {
	if v > 0 {
		return v
	}
	return def
}

// mapInferParamsToPredictOptions converts our adapter params into go-llama.cpp options
func mapInferParamsToPredictOptions(params InferParams, threads int) []llama.PredictOption {
	po := []llama.PredictOption{
		llama.SetTokens(max(1, params.MaxTokens)),
		llama.SetThreads(max(1, threads)),
		llama.SetTopP(zf(params.TopP, llama.DefaultOptions.TopP)),
		llama.SetTopK(zn(params.TopK, llama.DefaultOptions.TopK)),
		llama.SetTemperature(zf(params.Temperature, llama.DefaultOptions.Temperature)),
		llama.SetPenalty(zf(params.RepeatPenalty, llama.DefaultOptions.Penalty)),
	}
	if params.Seed != 0 {
		po = append(po, llama.SetSeed(params.Seed))
	}
	if len(params.Stop) > 0 {
		po = append(po, llama.SetStopWords(params.Stop...))
	}
	return po
}
