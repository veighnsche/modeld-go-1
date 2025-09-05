package manager

import "context"

// InferenceAdapter abstracts the model runtime used by the Manager.
// Concrete implementations (e.g., llama.cpp) should satisfy this interface.
type InferenceAdapter interface {
	// Start prepares a session for inference with the given model path and parameters.
	Start(modelPath string, params InferParams) (InferSession, error)
}

// InferSession represents a single inference session (lifecycle of one request or a reusable context).
type InferSession interface {
	// Generate streams tokens for the given prompt. The onToken callback will be invoked
	// for each token. Implementations must return when the context is canceled.
	Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error)
	// Close releases any resources associated with the session.
	Close() error
}

// InferParams captures generation parameters passed to the adapter.
type InferParams struct {
	Temperature   float32
	TopP          float32
	TopK          int
	MaxTokens     int
	Stop          []string
	Seed          int
	RepeatPenalty float32
	// Backend-specific options (e.g., threads, ctx size) can be added later.
}

// FinalResult summarizes the generation after streaming.
type FinalResult struct {
	Content      string
	Usage        Usage
	FinishReason string
}

// Usage contains token accounting.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
