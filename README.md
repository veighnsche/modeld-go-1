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

## Documentation

- Overview: [docs/overview.md](docs/overview.md)
- Build & Run: [docs/build-and-run.md](docs/build-and-run.md)
- API Reference: [docs/api.md](docs/api.md)
- Testing (Go, Python, Cypress, testctl): [docs/testing.md](docs/testing.md)
- Run CI locally with act: [docs/ci-local.md](docs/ci-local.md)
- Deployment (systemd): [docs/deployment.md](docs/deployment.md)
- Metrics: [docs/metrics.md](docs/metrics.md)

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

## Testing

See [docs/testing.md](docs/testing.md) for Go unit tests, Python E2E, and Cypress UI testing, including local and CI workflows.
