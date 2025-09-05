# modeld-go (scaffold)

![CI](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci-go.yml/badge.svg)
[![codecov](https://codecov.io/gh/veighnsche/modeld-go-1/graph/badge.svg)](https://codecov.io/gh/veighnsche/modeld-go-1)

Minimal scaffold for a Go control-plane around llama.cpp:
- Model switching
- Readiness + events (SSE/WebSocket later)
- Clean HTTP API surface

---

# modeld-go

A lightweight control-plane service (Go 1.22+) to manage multiple preloaded llama.cpp model instances within a configurable VRAM budget and margin, and to serve inference requests over a clean HTTP API. Hot math stays in llama.cpp (C/C++); Go handles lifecycle, I/O, backpressure, and observability.

## Documentation

- Overview: [docs/overview.md](docs/overview.md)
- Build & Run: [docs/build-and-run.md](docs/build-and-run.md)
- API Reference: [docs/api.md](docs/api.md)
- Testing (Go, Python, Cypress, testctl): [docs/testing.md](docs/testing.md)
- Run CI locally with act: [docs/ci-local.md](docs/ci-local.md)
- Deployment (systemd): [docs/deployment.md](docs/deployment.md)
- Metrics: [docs/metrics.md](docs/metrics.md)

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
  - `pkg/types/api.go`
- `deploy/` — systemd unit template
  - `deploy/modeld.service`
- `scripts/` — dev helpers
  - `scripts/dev-run.sh`

## Building

Requirements: Go 1.22+

- `make build` — builds `bin/modeld`
- `make run` — runs `go run ./cmd/modeld`
- `make tidy` — `go mod tidy`
- `make lint` — runs golangci-lint (install with `make install-golangci-lint`)

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

### Swagger (OpenAPI) Docs

This project includes Swagger annotations and can serve a Swagger UI when built with the `swagger` build tag.

- Generate docs (outputs to `docs/`):
  - `make swagger-gen`

- Run with Swagger UI enabled:
  - `make swagger-run`
  - Open `http://localhost:8080/swagger/index.html`
  - JSON spec at `http://localhost:8080/swagger/doc.json`

- Build a swagger-enabled binary:
  - `make swagger-build`

Notes:
- Default builds do not include the Swagger UI routes. The `internal/httpapi/MountSwagger()` no-op is replaced by a UI mount when using `-tags=swagger`.
- The `Makefile` pins `swag` to a specific version for reproducible docs; CI also regenerates and verifies docs are up to date.

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
  - Shape (`pkg/types/api.go`):
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
  - Optional logging overrides (per-request):
    - Query: `?log=off|error|info|debug`
    - Headers: `X-Log-Level: off|error|info|debug`, `X-Log-Infer: 1`
  - Error status codes:
    - 404 when model not found
    - 429 when instance queue/backpressure limits are hit
    - 500 for unexpected errors

- `GET /healthz`
  - Liveness. Always `200 ok` if process is up.
    return ParseConfigWith(flag.CommandLine, os.Args[1:])
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

For instructions to run the CI workflow locally with `act`, see [docs/ci-local.md](docs/ci-local.md).

GitHub Actions runs on pushes and pull requests:

- Go job
  - Module hygiene: runs `go mod tidy` and fails if it changes `go.mod`/`go.sum`.
  - Formatting: fails if `gofmt` finds unformatted files.
  - Static analysis: `go vet` (default and with `-tags=swagger`).
  - Swagger docs: regenerates with a pinned `swag` version, validates `docs/` artifacts exist, and fails if generation produces a diff vs. the repo (keeps Swagger docs in sync).
  - Build: compiles default and `-tags=swagger` variants.
  - Tests: runs `go test` with coverage and race detector; enforces 80%+ coverage and uploads to Codecov.
  - Lint: runs `golangci-lint` via the official GitHub Action with a pinned version.

- Python E2E job
  - Matrix across Python 3.10, 3.11, 3.12.
  - Caches pip based on `tests/e2e_py/requirements.txt` and Python version.
  - Runs `pytest tests/e2e_py` and uploads JUnit XML and console logs on failure.

- Cypress E2E (mock) job
  - Installs Node + pnpm, installs dependencies, builds `web/` and serves via `pnpm -C web preview`.
  - Sets `CYPRESS_BASE_URL=http://localhost:5173` and `CYPRESS_USE_MOCKS=1`.
  - Waits for the preview server with `scripts/poll-url.js`, then runs `pnpm run test:e2e:run`.
  - Uploads screenshots/videos from `e2e/artifacts/` on failure.

- Cypress E2E (live API) job
  - In addition to the above, sets up Go and starts the API server on `:18080` with a temporary models directory.
  - Exposes `CYPRESS_API_HEALTH_URL`, `CYPRESS_API_READY_URL`, and `CYPRESS_API_STATUS_URL` for the tests.
  - Runs the same Cypress suite headlessly and uploads artifacts on failure.

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

---

## Visual Harness + Cypress E2E (Testing Add-on)

This repo includes a minimal React-based visual test harness and a Cypress E2E suite to validate end-to-end behavior of the HTTP API (UI → API → stream → UI). This harness is strictly for testing and is not a production UI.

### Structure

- `web/` — Vite + React Router multi-page harness
  - Pages:
    - `Infer` — original streaming infer UI and controls
    - `Health` — fetches and shows `/healthz`
    - `Ready` — fetches and shows `/readyz`
    - `Models` — fetches and shows `/models` (supports array or `{ models: [...] }`)
    - `Status` — fetches and shows `/status`
  - Top-level navigation links between pages
  - Uses environment variables (no hardcoded URLs) and exposes a set of `data-testid` elements for automation
- `e2e/` — Cypress E2E tests (e2e mode)
  - Config: `e2e/cypress.config.ts`
  - Specs: `e2e/specs/*.cy.ts`
  - Support: `e2e/support/`
- `scripts/cli/poll-url.js` — small Node helper to poll a URL until it returns an expected status
- `package.json` (root) — orchestration scripts (with placeholders to adapt to your environment)

### Web Harness

Environment variables (via `web/.env` or your shell):

- `VITE_API_BASE_URL` (e.g. `http://localhost:8080`)
- `VITE_HEALTH_PATH` (default `/healthz`)
- `VITE_READY_PATH` (default `/readyz`)
- `VITE_MODELS_PATH` (default `/models`)
- `VITE_STATUS_PATH` (default `/status`)
- `VITE_INFER_PATH` (default `/infer`)
- `VITE_SEND_STREAM_FIELD` (default `false`) — include `{ "stream": true }` in POST if your server requires it
- `VITE_USE_MOCKS` (`true|false`, default `false`) — mock mode bypasses network and emits a deterministic NDJSON sequence

Rendered test IDs in the SPA:

- `data-testid="mode"` — `mock|live`
- `data-testid="models-count"` — optional models count (calls `/models` on load)
- `data-testid="prompt-input"` — textarea for prompt
- `data-testid="model-input"` — optional model id (blank to use default)
- `data-testid="submit-btn"` — send request
- `data-testid="status"` — `idle|requesting|success|error`
- `data-testid="stream-log"` — NDJSON lines appended as they arrive
- `data-testid="result-json"` — full last response (best-effort) as JSON string
- `data-testid="latency-ms"` — measured end-to-end duration in milliseconds

### Cypress E2E

Env file (copyable example at `.env.test.example`):

- `CYPRESS_BASE_URL` — the harness URL (e.g., `http://localhost:5173` if using Vite preview default)
- `CYPRESS_API_HEALTH_URL` — API health URL (e.g., `http://localhost:8080/healthz`)
- `CYPRESS_API_READY_URL` — API ready URL (e.g., `http://localhost:8080/readyz`)
- `CYPRESS_API_STATUS_URL` — API status URL (e.g., `http://localhost:8080/status`)
- `CYPRESS_MAX_LATENCY_MS` — threshold for end-to-end latency (default 5000)
- `CYPRESS_USE_MOCKS` — when `"1"`/`true`, E2E skips live API checks and relies on mock streaming

Notes:
- Cypress automatically picks up any environment variables prefixed with `CYPRESS_` and makes them available as `Cypress.env('<NAME_WITHOUT_PREFIX>')`.
- For local runs you may also set `USE_MOCKS` in your shell and map it inside tests, but using the `CYPRESS_` prefix is recommended for CI.

Artifacts on failure are written to `e2e/artifacts/` (screenshots/videos), and the suite includes a task to save arbitrary text files for debugging.

Core spec: `e2e/specs/visual_infer.cy.ts`

- Visits `/`, optionally waits for API `healthz` (live mode)
- Types a prompt, optionally sets a model, clicks send
- Asserts status transitions to `requesting` then `success`
- Asserts `stream-log` contains ≥ 2 lines and final line indicates completion (contains `"done": true`)
- Ensures `result-json` is non-empty and parseable JSON, no console errors, and latency under threshold

Additional specs (best-effort):

- `ready_flow.cy.ts` — verifies `readyz` transitions after first infer (skipped in mock mode)
- `models_list.cy.ts` — verifies `/models` count renders if available
- `errors.cy.ts` — invalid model name surfaces error (expects UI `status=error` and error text)

### Orchestration Scripts (root `package.json`)

The following scripts are provided and ready to use:

- `dev:api` — `make run` (starts the Go API locally)
- `dev:web` — `pnpm -C web dev` (starts Vite dev server)
- `dev:all` — runs both concurrently
- `test:e2e:open` — polls configured URLs then opens Cypress
- `test:e2e:run` — polls configured URLs then runs Cypress headlessly

Notes:

- `scripts/cli/poll-url.js` is used before Cypress to avoid race conditions.
- Install once at repo root with pnpm workspaces: `pnpm install`.
- You can run package scripts in the `web/` workspace via pnpm filtering, e.g. `pnpm -C web build`.

### Go CLI Helper (`bin/testctl`)

A Go-based CLI orchestrates installs and tests across Go, Python, and Cypress. It auto-selects free ports and enforces a strict rule for UI tests: if host models exist in `~/models/llm`, `test web auto` runs against the live API; otherwise it runs in mock mode.

Examples:

```bash
# Build the CLI
make testctl-build

# Install dependencies
bin/testctl install all    # JS, Go, Python
bin/testctl install js
bin/testctl install go
bin/testctl install py

# Run tests
bin/testctl test go                 # go test ./... -v
bin/testctl test api:py             # pytest tests/e2e_py
bin/testctl test web mock           # Cypress UI (mock mode)
bin/testctl test web live:host      # Cypress UI (host models required)
bin/testctl test web auto           # strict auto mode (host models => live)
bin/testctl test all auto           # full suite (Go + Py + UI auto)
```

Notes:

- The CLI will try to enable pnpm via corepack if available; otherwise ensure pnpm is installed.
- Cypress baseUrl is set automatically via `CYPRESS_BASE_URL` to the dynamic preview port.

Shortcuts:

- `pnpm run cli` — runs `bin/testctl`.
- `make test-all` — installs via testctl and runs the full suite with `test all auto`.

#### Implementation structure (testctl)

The CLI is implemented as a thin entrypoint with reusable internal logic:

- `cmd/testctl/main.go` — minimal `main` that delegates to `internal/testctl.Main()`.
- `internal/testctl/` — all CLI logic lives here and is unit-testable:
  - `Run(args []string, cfg *Config) error` — dispatches subcommands.
  - `ParseConfigWith(fs *flag.FlagSet, args []string) (*Config, []string)` — parses flags/env with an injectable `FlagSet` for tests.
  - `ParseConfig()` — small wrapper over `ParseConfigWith` using `flag.CommandLine` and `os.Args[1:]`.
  - `MainWithArgs(args []string) int` — testable entrypoint that returns an exit code.
  - `Main() int` — production entrypoint used by `cmd/testctl`.

Testing the CLI logic (unit):

```bash
go test ./internal/testctl -v
```

These tests stub indirect function variables (e.g., installers, runners) and verify dispatch/flag parsing without spawning subprocesses.

### Quickstart

1. Install deps (pnpm workspaces):
   - `pnpm install`
2. Configure envs:
   - Copy `.env.test.example` → `.env.test` and edit as needed
   - Edit `web/.env` (or use `web/.env.mock` for mock mode)
3. Run everything:
   - `pnpm run dev:all` (fills in your placeholders)
   - Visit the harness at `CYPRESS_BASE_URL`
4. Run tests (load env first):
   - Bash/Zsh: `set -a; source .env.test; set +a`
   - Then run:
     - Headless: `pnpm run test:e2e:run`
     - Interactive: `pnpm run test:e2e:open`

All URLs/ports/paths are env-driven; there are no hardcoded values in the harness or tests.
