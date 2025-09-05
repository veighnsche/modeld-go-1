package types

// Model represents a discoverable or loadable LLM model on disk.
type Model struct {
	// Stable identifier for the model.
	// example: tinyllama-q4
	ID string `json:"id" example:"tinyllama-q4"`
	// Human-friendly name.
	// example: TinyLlama (Q4)
	Name string `json:"name" example:"TinyLlama (Q4)"`
	// Absolute path to the model file on disk.
	// example: /home/user/models/TinyLlama.Q4_K_M.gguf
	Path string `json:"path" example:"/home/user/models/TinyLlama.Q4_K_M.gguf"`
	// Quantization level or variant string.
	// example: Q4_K_M
	Quant string `json:"quant" example:"Q4_K_M"`
	// Optional family (e.g., llama, mistral, phi).
	// example: llama
	Family string `json:"family,omitempty" example:"llama"`
}
