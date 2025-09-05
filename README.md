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

## Build modes and llama.cpp runtime

There are two build modes. The default is a no-CGO stub for fast development and CI. The in‑process llama runtime is available via build tags.

- Stub (default, no CGO):
  - No native dependencies. Useful for API/dev/CI.
  - Files: `internal/manager/adapter_llama_stub.go`.

- In‑process llama (CGO):
  - Enabled with: `-tags=llama`.
  - Uses `github.com/go-skynet/go-llama.cpp` to run models in‑process.
  - Files: `internal/manager/adapter_llama.go`, `internal/manager/llama_cgo.go`.

Notes

- External `llama_server` process mode has been removed. All inference goes through the in‑process go‑llama.cpp adapter.
- `llama_cgo.go` sets an rpath of `$ORIGIN` so the loader can find `libllama.so` next to your built binary, if you place it there.

### Enabling in‑process llama

1) Build with the llama tag:

```bash
go build -tags=llama ./cmd/modeld
```

2) Provide a config that enables inference and sets llama parameters (example snippet):

```go
mgr := manager.NewWithConfig(manager.ManagerConfig{
    Registry: []types.Model{{ID: "tinyllama-q4", Path: "/path/to/model.gguf"}},
    DefaultModel:     "tinyllama-q4",
    RealInferEnabled: true, // enables in‑process inference via go‑llama.cpp
    LlamaCtx:         4096,
    LlamaThreads:     8,
})
```

3) Ensure the native llama library is discoverable at runtime. Options:

- Place `libllama.so` next to the built binary (benefits from `$ORIGIN` rpath set by `llama_cgo.go`).
- Or install it in a system path that the dynamic loader uses.

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

- __How do I enable in‑process llama?__
  Build with `-tags=llama` and set `RealInferEnabled: true` in `manager.ManagerConfig`. Also set `LlamaCtx` and `LlamaThreads` as needed.

- __I get "cannot find -lllama" or runtime loader errors about `libllama.so`. What do I do?__
  Ensure the native llama library is available to the dynamic loader. Easiest is to place `libllama.so` next to your built binary; `internal/manager/llama_cgo.go` sets rpath `$ORIGIN` so the loader finds it there. Alternatively, install it in a system library path.

- __Do I need an external `llama_server` process?__
  No. External server mode was removed. All inference is handled in‑process via `go-skynet/go-llama.cpp`.

- __Builds succeed but inference returns a dependency error.__
  Make sure you built with `-tags=llama` and initialized the manager with `RealInferEnabled: true`. Without the tag, the stub adapter is used (no CGO) and inference is disabled.

- __How do I configure the default model?__
  Provide your registry via `manager.ManagerConfig.Registry` and set `DefaultModel` to the desired model ID. The `Path` field must point to a valid `.gguf` file.

- __How does streaming work?__
  `POST /infer` streams NDJSON lines. For the adapter, tokens are forwarded as they are generated. A final line includes `done: true` and simple usage info.

- __How is VRAM usage enforced?__
  The manager estimates model size from the file size (MB) and evicts least‑recently‑used idle instances when `BudgetMB` would be exceeded (plus `MarginMB`). See `internal/manager/instance_evict.go`.

- __Can I run tests without CGO?__
  Yes. By default (without `-tags=llama`) the stub adapter builds and all manager tests run with no native dependencies.

## License

GPL-3.0-only.

See `LICENSE` for the full text. When adding new source files, you can include an SPDX header like:

```text
// SPDX-License-Identifier: GPL-3.0-only
```

---

## Testing

See [docs/testing.md](docs/testing.md) for Go unit tests, Python E2E, and Cypress UI testing, including local and CI workflows.
