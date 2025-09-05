# Overview

`modeld-go` is a lightweight control-plane service (Go 1.22+) to manage multiple preloaded llama.cpp model instances within a configurable VRAM budget and margin, and to serve inference requests over a clean HTTP API.

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

See also:
- Build and run: `docs/build-and-run.md`
- API reference: `docs/api.md`
- Testing (Go, Python, Cypress, testctl): `docs/testing.md`
- CI locally with act: `docs/ci-local.md`
- Deployment (systemd): `docs/deployment.md`
- Metrics: `docs/metrics.md`
