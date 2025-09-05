# modeld-go

![CI](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci-go.yml/badge.svg)
[![codecov](https://codecov.io/gh/veighnsche/modeld-go-1/graph/badge.svg)](https://codecov.io/gh/veighnsche/modeld-go-1)
[![CI (E2E Cypress UI)](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci-e2e-cypress.yml/badge.svg)](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci-e2e-cypress.yml)
[![CI (E2E Python)](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci-e2e-python.yml/badge.svg)](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci-e2e-python.yml)
[![CI (E2E Cypress UI + Live API)](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci-e2e-cypress-live.yml/badge.svg)](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci-e2e-cypress-live.yml)

Production-ready Go control-plane for llama.cpp:
- Model switching and lifecycle management
- Health/ready probes and streaming responses (NDJSON)
- Clean HTTP API surface and basic observability

---

## Overview

A lightweight control-plane service (Go 1.23+) to manage multiple preloaded llama.cpp model instances within a configurable VRAM budget and margin, and to serve inference requests over a clean HTTP API. Hot math stays in llama.cpp (C/C++); Go handles lifecycle, I/O, backpressure, and observability.

## Quickstart

- Prereqs: Go 1.23+, Node.js 20+ (for optional web harness), pnpm
- Build & Run the API: see [docs/build-and-run.md](docs/build-and-run.md)
- API reference (Swagger): see [docs/api.md](docs/api.md)

## Runtime and llama.cpp server

The service uses an external `llama-server` process via HTTP (OpenAI-compatible endpoints). No CGO is required.

- Build:
  - `make build` produces a static Go binary (CGO disabled).
- Run llama.cpp server separately, for example:

```bash
llama-server \
  -m /path/to/model.gguf \
  -c 4096 -t 4 -ngl 0 \
  --host 127.0.0.1 --port 8081
```

- Run modeld and point it at the server:

```bash
./bin/modeld \
  --addr :8080 \
  --models-dir "$HOME/models/llm" \
  --default-model tinyllama-q4 \
  --llama-url http://127.0.0.1:8081 \
  --llama-timeout 30s \
  --llama-connect-timeout 5s \
  --llama-use-openai
```

Relevant flags (also supported in config files):

- `--llama-url` (string): Base URL of `llama-server`.
- `--llama-api-key` (string): Optional bearer token.
- `--llama-timeout` (duration): Request timeout (default 30s).
- `--llama-connect-timeout` (duration): Dial timeout (default 5s).
- `--llama-use-openai` (bool): Prefer OpenAI-compatible endpoints (default true).

See `docs/env-examples/llama-server.env.example` for a quick-start environment sample.

## API Cheat Sheet

Base URL examples:
- Local: `http://localhost:8080`

Health and readiness:

```bash
curl -sS http://localhost:8080/healthz
# ok

curl -i http://localhost:8080/readyz
# 200 OK with body "ready" when ready, or 503 with body "loading"
```

List models:

```bash
curl -sS http://localhost:8080/models | jq .
# {
#   "models": [ { ... }, ... ]
# }
```

Server status:

```bash
curl -sS http://localhost:8080/status | jq .
# {
#   "instances": [ { "model_id": "...", "state": "ready", ... } ],
#   "budget_mb": 8192, "used_est_mb": 2048, ...
# }
```

Inference (streams NDJSON):

```bash
curl -N \
  -H 'Content-Type: application/json' \
  -d '{
        "prompt": "Write a haiku about the ocean.",
        "model": "tinyllama-q4",
        "stream": true,
        "max_tokens": 64,
        "temperature": 0.7,
        "top_p": 0.9,
        "stop": ["\n\n"]
      }' \
  http://localhost:8080/infer
# Example NDJSON lines (shape may vary):
# {"token":"The","done":false}
# {"token":" ocean","done":false}
# ...
# {"done":true,"stats":{"duration_ms":1234}}
```

Optional per-request logging overrides:
- Query: `?log=off|error|info|debug`
- Headers: `X-Log-Level: off|error|info|debug`, `X-Log-Infer: 1`

Metrics (Prometheus):

```bash
curl -sS http://localhost:8080/metrics | head -n 20
# Prometheus text exposition format
```

Error responses (JSON):

```json
{"error":"invalid JSON body","code":400}
```

## HTTP API configuration and Swagger

You can configure the HTTP layer via an `Options` struct and the `NewMuxWithOptions` constructor in `internal/httpapi/`.

```go
svc := myServiceImplementation{}
opt := httpapi.Options{
    MaxBodyBytes:        2 << 20, // 2 MiB
    InferTimeoutSeconds: 60,      // per-request timeout for /infer
    CORSEnabled:         true,
    CORSAllowedOrigins:  []string{"*"},
    CORSAllowedMethods:  []string{"GET", "POST", "OPTIONS"},
    CORSAllowedHeaders:  []string{"Content-Type", "X-Log-Level"},
    Logger:              &myZerologLogger,
    BaseContext:         context.Background(),
}
handler := httpapi.NewMuxWithOptions(svc, opt)
srv := &http.Server{Addr: ":8080", Handler: handler}
```

Metrics path labels are normalized to chi route patterns (e.g. `/infer`) to keep Prometheus label cardinality low.

To enable Swagger UI and JSON (served under `/swagger/*`), build with the `swagger` tag:

```bash
go build -tags=swagger ./cmd/modeld
```

When built with `-tags=swagger`, the routes are:
- `/swagger/index.html` (UI)
- `/swagger/doc.json` (OpenAPI spec)

## Documentation

- Overview: [docs/overview.md](docs/overview.md)
- Build & Run: [docs/build-and-run.md](docs/build-and-run.md)
- API Reference: [docs/api.md](docs/api.md)
- Testing (Go, Python, Cypress, testctl): [docs/testing.md](docs/testing.md)
- Run CI locally with act: [docs/ci-local.md](docs/ci-local.md)
- Deployment (systemd): [docs/deployment.md](docs/deployment.md)
- Metrics: [docs/metrics.md](docs/metrics.md)

## Internal manager ergonomics

The orchestration layer lives in `internal/manager/`.

- Core types and config: `manager.go`, `types.go`, `config.go`
- Inference entry point: `inference.go`
- Instance lifecycle: `instance_ensure.go`, `instance_evict.go`
- Queueing: `queue_admission.go`
- Status: `status_report.go`
- Ops/demo: `ops_switch.go`
- Adapter interface: `adapter_iface.go`
- Llama adapters:
  - Real (tagged `llama`): `adapter_llama.go`, plus link hints in `llama_cgo.go`
  - Stub (no tag): `adapter_llama_stub.go`

Design goals:

- Single abstraction for runtimes via `InferenceAdapter`.
- No external `llama_server` processes; all real inference is in‑process via go‑llama.cpp.
- Keep package concerns small and discoverable by file naming.

- No authentication, quotas, or rate limits yet
- Basic FIFO per-model queues only
- SSE/WebSocket eventing may be added later; current streaming is NDJSON
- Metrics/tracing to be added later

## FAQ

- __How do I run inference?__
  Start `llama-server` with your `.gguf` model, then run `modeld` with `--llama-url` pointing at it. See the run examples above.

- __Do I need CGO or `libllama.so`?__
  No. The in-process adapter was removed. All inference goes through the external `llama-server` over HTTP.

- __Builds succeed but `/infer` returns a dependency error.__
  Ensure `--llama-url` (or `llama_url` in config) is set and the server is reachable. The preflight will fail clearly if not.

- __How do I configure the default model?__
  Provide your registry via `manager.ManagerConfig.Registry` and set `DefaultModel` to the desired model ID. The `Path` field must point to a valid `.gguf` file.

- __How does streaming work?__
  `POST /infer` streams NDJSON lines. For the adapter, tokens are forwarded as they are generated. A final line includes `done: true` and simple usage info.

- __How is VRAM usage enforced?__
  The manager estimates model size from the file size (MB) and evicts least‑recently‑used idle instances when `BudgetMB` would be exceeded (plus `MarginMB`). See `internal/manager/instance_evict.go`.

- __Can I run tests locally?__
  Yes. E2E tests run against a mock `llama-server`. Unit tests include an SSE streaming path for the adapter.

## License

GPL-3.0-only.

See `LICENSE` for the full text. When adding new source files, you can include an SPDX header like:

```text
// SPDX-License-Identifier: GPL-3.0-only
```

---

## Testing

See [docs/testing.md](docs/testing.md) for Go unit tests, Python E2E, and Cypress UI testing, including local and CI workflows.
