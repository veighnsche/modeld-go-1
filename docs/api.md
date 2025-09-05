# HTTP API

The server exposes a simple JSON over HTTP API. Default base URL is `http://localhost:8080` (configurable via `--addr`).

## Endpoints

- `GET /healthz`
  - Liveness probe. Returns `200 ok` if the process is up.

- `GET /readyz`
  - Readiness probe. Returns `200 ready` once at least one instance is ready (or the default route is ready); otherwise `503 loading`.

- `GET /models`
  - Returns the discovered registry of models.
  - Example:
    ```bash
    curl -s http://localhost:8080/models | jq
    ```

- `GET /status`
  - Returns instance summaries and VRAM budgeting info.
  - Shape (see `pkg/types/api.go`):
    ```go
    type InstanceStatus struct {
        ModelID       string `json:"model_id"`
        State         string `json:"state"`
        LastUsed      int64  `json:"last_used_unix"`
        EstVRAMMB     int    `json:"est_vram_mb"`
        QueueLen      int    `json:"queue_len"`
        Inflight      int    `json:"inflight"`
        MaxQueueDepth int    `json:"max_queue_depth"`
    }
    
    type StatusResponse struct {
        Instances      []InstanceStatus `json:"instances"`
        BudgetMB       int              `json:"budget_mb"`
        UsedMB         int              `json:"used_est_mb"`
        MarginMB       int              `json:"margin_mb"`
        Error          string           `json:"error,omitempty"`
        LastError      string           `json:"last_error,omitempty"`
        UptimeSeconds  int64            `json:"uptime_seconds"`
        ServerTimeUnix int64            `json:"server_time_unix"`
        EvictionsTotal uint64           `json:"evictions_total"`
        LoadsTotal     uint64           `json:"loads_total"`
    }
    ```

- `POST /infer` (Content-Type: `application/json`, Response: `application/x-ndjson`)
  - Request body (`pkg/types.InferRequest`):
    ```json
    { "model": "llama-3.1-8b-q4_k_m.gguf", "prompt": "Hello, world", "stream": true }
    ```
  - If `model` is omitted, the server uses the configured default model.
  - Response streams NDJSON lines; each line is a JSON object.

### NDJSON Streaming Schema

Adapters normalize their streaming outputs to a unified NDJSON contract for the HTTP layer:

- Token lines (zero or more):
  ```json
  { "token": "partial text" }
  ```
- Final line (exactly one):
  ```json
  {
    "done": true,
    "content": "full concatenated content (if adapter didn't supply a final content, this is built from tokens)",
    "finish_reason": "stop|length|...",
    "usage": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 }
  }
  ```

Notes:
- The `usage` object is adapter-reported when available; if unknown, it may be omitted or zeroed.
- This unified NDJSON schema remains stable across runtime adapters.

## Types reference

See `pkg/types/api.go` for DTOs:

- `InferRequest`
- `ModelsResponse`
- `ErrorResponse`
- `InstanceStatus`
- `StatusResponse`
