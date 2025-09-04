package types

// InferRequest represents an inference request payload.
type InferRequest struct {
    Model  string `json:"model,omitempty"`
    Prompt string `json:"prompt"`
    Stream bool   `json:"stream,omitempty"`
}

// InstanceStatus summarizes a loaded instance for /status.
type InstanceStatus struct {
    ModelID    string `json:"model_id"`
    State      string `json:"state"`
    LastUsed   int64  `json:"last_used_unix"`
    EstVRAMMB  int    `json:"est_vram_mb"`
    QueueLen   int    `json:"queue_len"`
    Inflight   int    `json:"inflight"`
    MaxQueueDepth int `json:"max_queue_depth"`
}

// StatusResponse is returned by GET /status.
type StatusResponse struct {
    Instances []InstanceStatus `json:"instances"`
    BudgetMB  int              `json:"budget_mb"`
    UsedMB    int              `json:"used_est_mb"`
    MarginMB  int              `json:"margin_mb"`
    Error     string           `json:"error,omitempty"`
}
