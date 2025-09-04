# PROJECT REQUIREMENTS — modeld-go

## Goal
Build a **control-plane service** (Go 1.22+) that can manage **multiple preloaded llama.cpp instances** (one context per model) within a configured **VRAM budget** and **safety margin**, and serves LLM requests. The target model is selected per request via an optional `model` parameter. There is no separate model-switch endpoint and no explicit "ready after switch" eventing; readiness is reflected via probes and API responses. Keep hot math in llama.cpp (C/C++). Go handles lifecycle, I/O, and observability.

## MVP Scope (this phase)
- Single process, potentially **multiple active model instances** preloaded concurrently.
- Endpoints (shape only; responses can be stubbed initially):
  - `GET /models` → list discovered models from `--models-dir` (id = full filename including extension, name = same as id, path = absolute file path).
  - `POST /infer` (streaming) → accepts JSON body and returns a streaming LLM response, routing to the requested or default model.
    - Request JSON fields (initial):
      - `model` (optional, string): target model id. The id is the full filename including the `.gguf` extension (e.g., `llama-3.1-8b-q4_k_m.gguf`). If omitted, system uses the current model.
      - `prompt` (string): user prompt.
      - `stream` (bool, default true): stream tokens back as they are generated.
    - Behavior:
      - If `model` is omitted: do not change instances; generate using the configured default model.
      - If `model` equals an already-loaded instance: reuse it; generate immediately.
      - If `model` differs and is not loaded: load that model (respecting VRAM budget + margin; evict as needed), then generate.
      - If `model` is not found in the `--models-dir` scan, return `404 Not Found`.
  - Probes:
    - `GET /healthz` → liveness (always 200).
    - `GET /readyz` → readiness: 200 when at least one instance is ready (or when default route is ready); 503 otherwise.
  - Manager behavior:
  - Discover **registry** by scanning a directory (`--models-dir`, default `~/models/llm`) for `*.gguf` files at startup. No YAML metadata is required. IDs are the full filenames including extension.
  - Maintain **instances**: `model_id -> {state, last_used, est_vram_mb}`.
  - Enforce **VRAM budget** and **margin** (configurable). When loading a new instance would exceed the budget, **evict** least-recently-used idle instances until it fits.
  - Per-instance state machine: `ready → loading → ready|error`.
  - Cancellation: cancel in-flight generations on instance unload/evict (stub for now).
  - Config: models are discovered from `--models-dir`; hot-reload (directory watch) can be added later.

## Non-Goals (for later phases)
- Blue/green zero-downtime (second ctx pre-warm then swap).
- Multi-model concurrency / pool.
- Auth, rate-limit, quotas.
- Rich metrics/tracing.

## Streaming Contract
- Responses from `POST /infer` stream tokens as they are generated.
- Transport TBD: SSE or chunked JSON lines (ndjson). Keep implementation simple for MVP.

## Queuing and Backpressure (MVP)
- Per-instance concurrency: 1 in-flight generation per model instance (others queue).
- Queue policy: FIFO per target model. Separate queues per model id.
- Queue limits:
  - `max-queue-depth` per model (default: 32; configurable later).
  - `max-wait-ms` per request (default: 30000ms). If exceeded, return `429 Too Many Requests` with a retry hint.
- Global guardrails:
  - Optional global `max-inflight` across all instances to protect CPU/GPU.
  - If global limit reached, return `429` early.
- Cancellation & client disconnects:
  - If the client disconnects while queued, remove the request.
  - If disconnects during streaming, cancel generation promptly.
- Fairness:
  - Basic FIFO is sufficient for MVP. No per-tenant fairness or priorities in MVP.

## Operational Requirements
- Single static binary; systemd service file template in `deploy/`.
- Health endpoints: `/healthz` (liveness, always 200), `/readyz` (readiness: at least one instance ready).
- Log format: structured JSON lines (later), info level by default.
- CLI/ENV config (initial):
  - `--models-dir` (string): directory to scan for `*.gguf` model files (default `~/models/llm`). IDs are filename stems.
  - `--vram-budget-mb` (int): max VRAM to use for all instances combined.
  - `--vram-margin-mb` (int): reserved headroom to keep free.
  - `--default-model` (string, optional): default route when `model` omitted.

## Performance Guardrails
- No per-token cgo chatter (batch or ring-buffer when we wire llama).
- Streaming with minimal allocations.
- Warmup 1 token after load before declaring ready.

## Code Structure
- `cmd/modeld/` → wires HTTP server + manager.
- `internal/manager/` → multi-instance lifecycle, state machine, VRAM budgeting & eviction.
- `internal/llm/` → adapter interface; later binds to llama.cpp.
- `internal/httpapi/` → handlers, routing.
- `pkg/types/` → shared DTOs for requests/events and instance status.
- Models live on the filesystem under the directory provided by `--models-dir`.
- `deploy/` → systemd unit template.
- `scripts/` → dev helpers (lint/build/run).

## Build Targets (Makefile)
- `make build` → builds to `bin/modeld`
- `make run` → `go run ./cmd/modeld`
- `make tidy` → `go mod tidy`
