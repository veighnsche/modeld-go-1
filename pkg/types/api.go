package types

// InferRequest represents an inference request payload.
type InferRequest struct {
	// Optional model identifier. If empty, the server default is used.
	// example: tinyllama-q4
	Model string `json:"model,omitempty" example:"tinyllama-q4"`
	// Required prompt text to generate a completion for.
	// example: Write a haiku about the ocean.
	Prompt string `json:"prompt" example:"Write a haiku about the ocean."`
	// If true, stream results as NDJSON tokens. When false, the server may still stream internally but buffer.
	// example: true
	Stream bool `json:"stream,omitempty" example:"true"`
	// Maximum number of new tokens to generate.
	// example: 128
	MaxTokens int `json:"max_tokens,omitempty" example:"128"`
	// Sampling temperature (higher = more random).
	// example: 0.7
	Temperature float64 `json:"temperature,omitempty" example:"0.7"`
	// Nucleus sampling probability.
	// example: 0.9
	TopP float64 `json:"top_p,omitempty" example:"0.9"`
	// Top-K sampling: limit candidates to top K tokens.
	// example: 40
	TopK int `json:"top_k,omitempty" example:"40"`
	// Optional stop sequences. Generation stops when any sequence is matched.
	// example: ["\n\n","END"]
	Stop []string `json:"stop,omitempty" example:"[\"\\n\\n\",\"END\"]"`
	// Random seed for reproducibility; 0 or omitted lets the server choose.
	// example: 42
	Seed int64 `json:"seed,omitempty" example:"42"`
	// Repeat penalty applied by some llama servers.
	// example: 1.1
	RepeatPenalty float64 `json:"repeat_penalty,omitempty" example:"1.1"`
}

// ModelsResponse wraps the list of models returned by GET /models.
type ModelsResponse struct {
	// List of available models.
	Models []Model `json:"models"`
}

// ErrorResponse is a consistent JSON error payload.
type ErrorResponse struct {
	// Error message.
	// example: invalid JSON body
	Error string `json:"error" example:"invalid JSON body"`
	// HTTP status code.
	// example: 400
	Code int `json:"code" example:"400"`
}

// InstanceStatus summarizes a loaded instance for /status.
type InstanceStatus struct {
	// ID of the model this instance serves.
	// example: tinyllama-q4
	ModelID string `json:"model_id" example:"tinyllama-q4"`
	// Current lifecycle state of the instance (e.g., unloaded, loading, ready).
	// example: ready
	State string `json:"state" example:"ready"`
	// Last time this instance served a request (unix seconds).
	// example: 1700000000
	LastUsed int64 `json:"last_used_unix" example:"1700000000"`
	// Estimated VRAM usage in MB.
	// example: 1200
	EstVRAMMB int `json:"est_vram_mb" example:"1200"`
	// Current queue length for incoming requests.
	// example: 0
	QueueLen int `json:"queue_len" example:"0"`
	// Number of in-flight requests currently being processed.
	// example: 1
	Inflight int `json:"inflight" example:"1"`
	// Maximum queued requests allowed before backpressure triggers.
	// example: 32
	MaxQueueDepth int `json:"max_queue_depth" example:"32"`
	// TCP port used by the managed runtime (when spawn mode is active).
	// example: 30001
	Port int `json:"port,omitempty" example:"30001"`
	// Process ID of the managed runtime (when spawn mode is active).
	// example: 12345
	PID int `json:"pid,omitempty" example:"12345"`
}

// StatusResponse is returned by GET /status.
type StatusResponse struct {
	// Loaded/managed instances.
	Instances []InstanceStatus `json:"instances"`
	// VRAM budget in MB across all instances.
	// example: 8192
	BudgetMB int `json:"budget_mb" example:"8192"`
	// Estimated used VRAM in MB.
	// example: 2048
	UsedMB int `json:"used_est_mb" example:"2048"`
	// Reserved VRAM margin in MB.
	// example: 512
	MarginMB int `json:"margin_mb" example:"512"`
	// Optional top-level error message.
	Error string `json:"error,omitempty"`
	// Last error observed by the manager (if any).
	LastError string `json:"last_error,omitempty"`
	// Uptime of the server in seconds.
	// example: 3600
	UptimeSeconds int64 `json:"uptime_seconds" example:"3600"`
	// Server time in unix seconds.
	// example: 1700000000
	ServerTimeUnix int64 `json:"server_time_unix" example:"1700000000"`
	// Total number of evictions performed to free VRAM.
	// example: 5
	EvictionsTotal uint64 `json:"evictions_total" example:"5"`
	// Total number of model loads.
	// example: 12
	LoadsTotal uint64 `json:"loads_total" example:"12"`
    // Overall manager state (e.g., loading, ready, error).
    // example: ready
    State string `json:"state" example:"ready"`
    // Number of instances currently warming up (loading).
    // example: 1
    WarmupsInProgress int `json:"warmups_in_progress" example:"1"`
    // Number of instances currently draining (unload in progress).
    // example: 1
    DrainingCount int `json:"draining_count" example:"1"`
}
