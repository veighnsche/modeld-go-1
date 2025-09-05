package llm

import "context"

// Adapter defines the minimal interface for an LLM backend integration.
// It is a forward-looking placeholder for future llama.cpp (or similar) bindings.
// Keep this surface small; heavy lifting should remain in native code.
// Adapter is a future interface over llama.cpp.
// Keep it tiny; hot math stays in C/C++.
type Adapter interface {
	Load(path string) error
	Unload() error
	Warmup(ctx context.Context) error
	// Generate(ctx, prompt) (stream) -- later
}
