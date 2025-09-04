# PROJECT REQUIREMENTS — modeld-go

## Goal
Build a **control-plane service** (Go 1.22+) that manages a single active llama.cpp context with **model switching**, **front-end visibility** (which model is loaded), and **clear readiness signals** after a switch. Keep hot math in llama.cpp (C/C++). Go handles lifecycle, I/O, and observability.

## MVP Scope (this phase)
- Single-process, one active model context at a time.
- Endpoints (shape only; responses can be stubbed initially):
  - `GET /models` → list registry (id, name, path, quant, family).
  - `POST /switch {model_id}` → 202 + op_id (switch happens async).
  - `GET /status` → `{ state: ready|loading|error, current_model, error? }`.
  - `GET /events` (SSE) → `model_switch_started`, `model_loading_progress`, `model_ready`, `model_error`.
  - `GET /healthz` → 200 when ready, 503 otherwise.
- State machine: `ready → loading → ready|error` (serialize switches).
- Cancellation: cancel in-flight generations on switch (stub for now).
- Config: `configs/models.yaml` defines registry; hot-reload later.

## Non-Goals (for later phases)
- Blue/green zero-downtime (second ctx pre-warm then swap).
- Multi-model concurrency / pool.
- Auth, rate-limit, quotas.
- Rich metrics/tracing.

## Event Contract (SSE payloads)
- `model_switch_started` `{ op_id, target_model_id }`
- `model_loading_progress` `{ op_id, step: "unload"|"mmap"|"warmup", pct }`
- `model_ready` `{ op_id, current_model_id }`
- `model_error` `{ op_id, message }`

## Operational Requirements
- Single static binary; systemd service file template in `deploy/`.
- Health endpoints: `/livez` (always 200), `/readyz` (same as `/healthz`).
- Log format: structured JSON lines (later), info level by default.

## Performance Guardrails
- No per-token cgo chatter (batch or ring-buffer when we wire llama).
- Streaming over SSE with minimal allocations.
- Warmup 1 token after load before declaring ready.

## Code Structure
- `cmd/modeld/` → wires HTTP server + manager.
- `internal/manager/` → lifecycle & state machine (no llama details).
- `internal/llm/` → adapter interface; later binds to llama.cpp.
- `internal/httpapi/` → handlers, routing, SSE hub.
- `pkg/types/` → shared DTOs for requests/events.
- `configs/` → model registry YAML.
- `deploy/` → systemd unit template.
- `scripts/` → dev helpers (lint/build/run).

## Build Targets (Makefile)
- `make build` → builds to `bin/modeld`
- `make run` → `go run ./cmd/modeld`
- `make tidy` → `go mod tidy`

