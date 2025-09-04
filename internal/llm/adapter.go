package llm

import "context"

// Adapter is a future interface over llama.cpp.
// Keep it tiny; hot math stays in C/C++.
type Adapter interface {
Load(path string) error
Unload() error
Warmup(ctx context.Context) error
// Generate(ctx, prompt) (stream) -- later
}
