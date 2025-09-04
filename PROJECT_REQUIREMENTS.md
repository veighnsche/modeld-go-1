# PROJECT REQUIREMENTS — modeld-go

## Goal
Build a **control-plane service** (Go 1.22+) that manages a single active llama.cpp context and serves LLM requests. The active model can change implicitly per request based on an optional `model` parameter in the request. There is no separate "model switch" endpoint and no explicit "ready after switch" signaling. Keep hot math in llama.cpp (C/C++). Go handles lifecycle, I/O, and observability.

## MVP Scope (this phase)
- Single-process, one active model context at a time.
- Endpoints (shape only; responses can be stubbed initially):
  - `GET /models` → list registry (id, name, path, quant, family).
  - `POST /infer` (streaming) → accepts JSON body and returns a streaming LLM response.
    - Request JSON fields (initial):
      - `model` (optional, string): target model id. If omitted, system uses the currently loaded model.
      - `prompt` (string): user prompt.
      - `stream` (bool, default true): stream tokens back as they are generated.
    - Behavior:
      - If `model` is omitted: do not change the current model; generate using the currently loaded model.
      - If `model` equals the current model: do not change; generate immediately.
      - If `model` differs from the current model: switch to the requested model, then generate.
  - Probes:
    - `GET /healthz` → liveness (always 200).
    - `GET /readyz` → readiness of the currently loaded model (200 when a model is loaded and warmed; 503 otherwise).
- There is NO dedicated `POST /switch` endpoint and NO SSE event stream for switch lifecycle.
- State machine: internal only; expose readiness via `/readyz` and errors via API responses.
- Cancellation: cancel in-flight generations on implicit switch (stub for now).
- Config: `configs/models.yaml` defines registry; hot-reload later.

## Non-Goals (for later phases)
- Blue/green zero-downtime (second ctx pre-warm then swap).
- Multi-model concurrency / pool.
- Auth, rate-limit, quotas.
- Rich metrics/tracing.

## Streaming Contract
- Responses from `POST /infer` stream tokens as they are generated.
- Transport TBD: SSE or chunked JSON lines (ndjson). Keep implementation simple for MVP.

## Operational Requirements
- Single static binary; systemd service file template in `deploy/`.
- Health endpoints: `/healthz` (liveness, always 200), `/readyz` (readiness for current model).
- Log format: structured JSON lines (later), info level by default.

## Performance Guardrails
- No per-token cgo chatter (batch or ring-buffer when we wire llama).
- Streaming over SSE with minimal allocations.
- Warmup 1 token after load before declaring ready.

## Code Structure
- `cmd/modeld/` → wires HTTP server + manager.
- `internal/manager/` → lifecycle & state machine (no llama details).
- `internal/llm/` → adapter interface; later binds to llama.cpp.
- `internal/httpapi/` → handlers, routing.
- `pkg/types/` → shared DTOs for requests/events.
- `configs/` → model registry YAML.
- `deploy/` → systemd unit template.
- `scripts/` → dev helpers (lint/build/run).

## Build Targets (Makefile)
- `make build` → builds to `bin/modeld`
- `make run` → `go run ./cmd/modeld`
- `make tidy` → `go mod tidy`
