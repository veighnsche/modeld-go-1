# modeld-go (scaffold)

![CI](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci.yml/badge.svg)
[![codecov](https://codecov.io/gh/veighnsche/modeld-go-1/graph/badge.svg)](https://codecov.io/gh/veighnsche/modeld-go-1)

Minimal scaffold for a Go control-plane around llama.cpp:
- Model switching
- Readiness + events (SSE/WebSocket later)
- Clean HTTP API surface

---

# modeld-go

A lightweight control-plane service (Go 1.22+) to manage multiple preloaded llama.cpp model instances within a configurable VRAM budget and margin, and to serve inference requests over a clean HTTP API. Hot math stays in llama.cpp (C/C++); Go handles lifecycle, I/O, backpressure, and observability.

## Features

- Multiple models discovered from a models directory (scans for .gguf)
- Per-request model routing with a configurable default
- VRAM budgeting with LRU eviction to make new loads fit
- Simple, streaming inference API (NDJSON)
- Health and readiness probes
- Single static binary, systemd-ready

## Project Structure

- `cmd/modeld/` — main entrypoint that wires flags/config, registry load, HTTP server
  - `cmd/modeld/main.go`
- `internal/httpapi/` — HTTP router and handlers
  - `internal/httpapi/server.go`
- `internal/manager/` — core lifecycle: instances, queues, VRAM budgeting, eviction
- `internal/config/` — config file loader supporting YAML/JSON/TOML
  - `internal/config/loader.go`
- `internal/llm/` — adapter interface for llama.cpp integration
  - `internal/llm/adapter.go`
- `pkg/types/` — shared request/response DTOs
  - `pkg/types/types.go`
- `deploy/` — systemd unit template
  - `deploy/modeld.service`
- `scripts/` — dev helpers
  - `scripts/dev-run.sh`

## Building

Requirements: Go 1.22+

- `make build` — builds `bin/modeld`
- `make run` — runs `go run ./cmd/modeld`
- `make tidy` — `go mod tidy`

## Running

You can configure via CLI flags and/or a config file. CLI flags take precedence over config values when explicitly provided.

Flags (see `cmd/modeld/main.go`):

- `--addr` (env: `MODELD_ADDR`), default `:8080`
- `--config` path to YAML/JSON/TOML config file (optional)
- `--models-dir` directory to scan for `*.gguf` (default `~/models/llm`)
- `--vram-budget-mb` integer VRAM budget across all instances (0 = unlimited)
- `--vram-margin-mb` integer VRAM margin to keep free
- `--default-model` default model id when omitted in requests

Cancellation behavior:

- Client disconnects during `POST /infer` will cancel the in-flight generation promptly.
- Graceful shutdown (SIGINT/SIGTERM) cancels in-flight and queued requests by propagating a base HTTP context through handlers.

Request requirements:

- `POST /infer` requires `Content-Type: application/json`.
- Request body is limited to ~1MiB for MVP to protect the server. Large prompts should be handled via files or future endpoints.

Example run:

```bash
go run ./cmd/modeld \
  --addr :8080 \
  --models-dir "$HOME/models/llm" \
  --vram-budget-mb 20000 \
  --vram-margin-mb 1024 \
  --default-model llama-3.1-8b-q4_k_m.gguf
```

## Configuration File

Config can be provided as YAML, JSON, or TOML. Keys are the same across formats and correspond to `internal/config/loader.go`:

```yaml
# config.yaml
addr: ":8080"
models_dir: "~/models/llm"
vram_budget_mb: 20000
vram_margin_mb: 1024
default_model: "llama-3.1-8b-q4_k_m.gguf"
```

```json
{
  "addr": ":8080",
  "models_dir": "~/models/llm",
  "vram_budget_mb": 20000,
  "vram_margin_mb": 1024,
  "default_model": "llama-3.1-8b-q4_k_m.gguf"
}
```

```toml
addr = ":8080"
models_dir = "~/models/llm"
vram_budget_mb = 20000
vram_margin_mb = 1024
default_model = "llama-3.1-8b-q4_k_m.gguf"
```

Use it via `--config /path/to/config.yaml`. Any CLI flag you set explicitly overrides matching config values.

## Model Registry

On startup, the service scans `--models-dir` for `*.gguf` to build a registry (see `cmd/modeld/main.go`). Model IDs are the full filenames including the `.gguf` extension. The `GET /models` endpoint returns the discovered models:

`pkg/types.Model`:

```go
type Model struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Path   string `json:"path"`
    Quant  string `json:"quant"`
    Family string `json:"family,omitempty"`
}
```

## Manager Overview

The manager orchestrates a map of model instances and enforces VRAM constraints (`internal/manager/`). Behavior (as used by the HTTP layer in `internal/httpapi/server.go`):

- Load requested model on demand; if already loaded, reuse it
- Maintain instance state, last used time, and estimated VRAM usage
- Enforce VRAM budget + margin; before a new load, evict least-recently-used idle instances until the new instance fits
- Provide readiness (`Ready()`) used by `/readyz`
- Provide status for `/status`
- Route inference and stream results

Error mapping done in `internal/httpapi/server.go`:

- Model not found → `404 Not Found`
- Too busy / queue overflow → `429 Too Many Requests`
- Other errors → `500 Internal Server Error`

## HTTP API

Router is built with `chi` (`internal/httpapi/server.go`). All responses are JSON unless otherwise stated.

Endpoints:

- `GET /models`
  - Returns the discovered registry of models.
  - Example:
    ```bash
    curl -s http://localhost:8080/models | jq
    ```

- `GET /status`
  - Returns instance summaries and VRAM budgeting info using `pkg/types.StatusResponse`/`InstanceStatus`.
  - Shape (`pkg/types/types.go`):
    ```go
    type InstanceStatus struct {
        ModelID   string `json:"model_id"`
        State     string `json:"state"`
        LastUsed  int64  `json:"last_used_unix"`
        EstVRAMMB int    `json:"est_vram_mb"`
        QueueLen  int    `json:"queue_len"`
    }
    
    type StatusResponse struct {
        Instances []InstanceStatus `json:"instances"`
        BudgetMB  int              `json:"budget_mb"`
        UsedMB    int              `json:"used_est_mb"`
        MarginMB  int              `json:"margin_mb"`
        Error     string           `json:"error,omitempty"`
    }
    ```
  - Example:
    ```bash
    curl -s http://localhost:8080/status | jq
    ```

- `POST /infer` (Content-Type: `application/json`, Response: `application/x-ndjson`)
  - Request body (`pkg/types.InferRequest`):
    ```json
    { "model": "llama-3.1-8b-q4_k_m.gguf", "prompt": "Hello, world", "stream": true }
    ```
  - If `model` omitted, the manager will use the configured default model.
  - Response streams NDJSON lines; each line is a JSON object. Example invocation:
    ```bash
    curl -N -X POST http://localhost:8080/infer \
      -H 'Content-Type: application/json' \
      -d '{"model":"llama-3.1-8b-q4_k_m.gguf","prompt":"Write a haiku about Go."}'
    ```
  - Error status codes:
    - 404 when model not found
    - 429 when instance queue/backpressure limits are hit
    - 500 for unexpected errors

- `GET /healthz`
  - Liveness. Always `200 ok` if process is up.

- `GET /readyz`
  - Readiness. `200 ready` when at least one instance is ready (or default route is ready); otherwise `503 loading`.

## LLM Adapter

`internal/llm/adapter.go` defines the tiny surface to bind llama.cpp:

```go
type Adapter interface {
    Load(path string) error
    Unload() error
    Warmup(ctx context.Context) error
    // Generate(ctx, prompt) (stream) — later
}
```

This keeps high-frequency math in C/C++ while managing lifecycle in Go.

## Deployment (systemd)

See `deploy/modeld.service` for a template unit:

```ini
[Unit]
Description=modeld-go (model manager)
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=%h/Projects/modeld-go-1/bin/modeld --config %h/Projects/modeld-go-1/configs/models.yaml --addr :8080
WorkingDirectory=%h/Projects/modeld-go-1
Restart=on-failure
Environment=GOMAXPROCS=0
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only

[Install]
WantedBy=multi-user.target
```

Adjust paths to match your environment and ensure your config file and models directory exist.

## Development

- `scripts/dev-run.sh` runs the server with an example `--config` path. Create that config file or change the script to pass your flags.

## Testing

There are two primary test suites:

- Go unit and in-process E2E tests
  - Command: `make test`
  - Location: `internal/**` (e.g., `internal/httpapi`, `internal/manager`, `internal/e2e`)
  - Notes: E2E tests construct an `httptest.Server` using the mux and manager. Shared helpers live in `internal/e2e/helpers_test.go`.

- Python black-box tests
  - Command: `make e2e-py`
  - Location: `tests/e2e_py/`
  - Helpers: `tests/e2e_py/helpers.py` provides `start_server`, `start_server_with_handle`, `start_server_with_config`, and utilities.
  - Notes: Tests build the binary (`CGO_ENABLED=0`) and start a subprocess on a free port, exercising the HTTP API over real sockets.

Common endpoints exercised include `/healthz`, `/readyz`, `/models`, `/status`, and streaming `POST /infer` (NDJSON). Negative-path tests cover 404 and backpressure (429) scenarios.

## Continuous Integration (CI)

GitHub Actions runs on pushes and pull requests:

- Go job
  - Runs `go test ./...` with coverage and enforces a minimum of 80%.
  - Uploads `coverage.out` and sends coverage to Codecov if `CODECOV_TOKEN` is configured.

- Python E2E job
  - Matrix across Python 3.10, 3.11, 3.12.
  - Caches pip based on `tests/e2e_py/requirements.txt` and Python version.
  - Runs `pytest tests/e2e_py` and uploads JUnit XML and console logs on failure.

## Metrics

The server exposes Prometheus metrics at `GET /metrics`.

Current metrics (namespace `modeld`, subsystem `http`):

- `modeld_http_requests_total` (counter)
  - Labels: `path`, `method`, `status`
- `modeld_http_request_duration_seconds` (histogram)
  - Labels: `path`, `method`, `status`
- `modeld_http_inflight_requests` (gauge)
  - Labels: `path`
- `modeld_http_backpressure_total` (counter)
  - Labels: `reason` (e.g., `queue_full`, `wait_timeout`)

Notes:
- The middleware instruments all HTTP handlers. Path labels currently use the request path string. Consider mapping to stable route names to reduce cardinality in high-variance environments.

- Unit tests for the manager live in `internal/manager/manager_test.go`.

## Roadmap / Non-Goals (initial)

- Not yet implementing llama.cpp bindings; `internal/llm` is a stub interface
- No authentication, quotas, or rate limits yet
- Basic FIFO per-model queues only
- SSE/WebSocket eventing may be added later; current streaming is NDJSON
- Metrics/tracing to be added later

## License

GPL-3.0-only.

See `LICENSE` for the full text. When adding new source files, you can include an SPDX header like:

```text
// SPDX-License-Identifier: GPL-3.0-only
```
