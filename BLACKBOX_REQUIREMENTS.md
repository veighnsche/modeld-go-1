# Black‑Box E2E Test Requirements

This document defines end‑to‑end (E2E) black‑box API test cases for the current HTTP surface of `modeld`.

The goal is to validate observable behavior by starting the real server process (or an equivalent `httptest` server) and exercising its public endpoints purely over HTTP.

## Scope

- Server entrypoint: `cmd/modeld/main.go`.
- Router and handlers: `internal/httpapi/server.go`.
- Manager behavior (as observable via API): `internal/manager/*`.
- Types and payloads: `pkg/types/*`.

## How to Run

- Locally: `make e2e-py`
  - Uses a virtualenv in `.venv`, installs `tests/e2e_py/requirements.txt`, and runs `pytest tests/e2e_py`.
- CI: GitHub Actions runs the suite on Python 3.10, 3.11, and 3.12 and uploads artifacts on failure.

## Test Environment Setup

- Create a temporary models directory populated with one or more empty `*.gguf` files (the registry loader only checks file name suffix for discovery).
- Start the server with:
  - `--addr` or `MODELD_ADDR` set to a known free port.
  - `--models-dir` pointing to the temp directory.
  - Optional: `--default-model` set to an existing file name (e.g., `alpha.gguf`).
  - Optional: `--vram-budget-mb` and `--vram-margin-mb` for budgeting paths.
- Define a simple readiness wait strategy:
  - `GET /healthz` should be `200` as soon as the process is up.
  - `GET /readyz` becomes `200` after at least one model instance reaches `Ready` state (typically triggered by performing an `/infer`).

Notes:
- For in‑process Go tests, `httptest.NewServer(httpapi.NewMux(manager.New(...)))` is already used in `internal/e2e/e2e_test.go`.
- Python black‑box helpers live in `tests/e2e_py/helpers.py` and provide `start_server`, `start_server_with_handle`, and `start_server_with_config` for YAML/JSON/TOML.

## Endpoints and Test Cases

### 1) GET /healthz (Liveness)

- [HAPPY] Returns `200 OK` with body containing `ok` once the HTTP server is accepting connections.
  - Precondition: server started.
  - Assert: `status=200`, `body` contains `ok`.

### 2) GET /readyz (Readiness)

- [INIT] Returns `503 Service Unavailable` with body containing `loading` if no model instance is yet ready.
  - Precondition: server started; no inference performed; default model may not be warmed.
  - Assert: `status=503`, `body` contains `loading`.

- [READY] Returns `200 OK` with body containing `ready` after at least one instance is warmed.
  - Action: POST `/infer` (see below) using an existing model or default model to trigger warmup.
  - Poll `/readyz` until `200`, within timeout.
  - Assert: `status=200`, `body` contains `ready`.

### 3) GET /models (Registry)

- [HAPPY] Returns `200 OK` and JSON list of discovered models.
  - Precondition: temp models dir contains N `*.gguf` files.
  - Assert: `status=200`, `Content-Type` includes `application/json`.
  - Assert: response JSON has key `models` with length N. Elements conform to `pkg/types.Model` shape.

- [EMPTY] Returns `200 OK` with empty list when directory has zero `*.gguf` files.
  - Precondition: empty directory.
  - Assert: `models` length is 0.

### 4) GET /status (Manager snapshot)

- [HAPPY] Returns `200 OK` and JSON body conforming to `pkg/types.StatusResponse`.
  - Assert: `status=200`, JSON decodes to `StatusResponse`.
  - If no inference yet: `Instances` may be empty; numeric fields present.

- [AFTER_INFER] After performing an `/infer`, returns at least one instance with plausible fields.
  - Action: POST `/infer` to warm an instance.
  - Assert: `Instances` length >= 1.
  - Optional: fields like `BudgetMB`, `UsedMB`, `MarginMB` are integers; `InstanceStatus` fields present.

### 5) POST /infer (Streaming NDJSON)

- [HAPPY_DEFAULT_MODEL] With payload omitting `model` and `--default-model` configured, returns `200 OK` and streams NDJSON.
  - Request JSON: `{ "prompt": "hello" }` (and optionally `"stream": true`).
  - Assert: `status=200`, `Content-Type` includes `application/x-ndjson`.
  - Assert: response body contains multiple newline‑delimited JSON objects (at least two lines; last contains a terminal marker like `{ "done": true }` in current stub).

- [HAPPY_EXPLICIT_MODEL] With `model` set to an existing id (e.g., `alpha.gguf`), returns `200 OK` streaming NDJSON.
  - Request JSON: `{ "model": "alpha.gguf", "prompt": "hi" }`.
  - Assert as above.

- [BAD_JSON] With invalid JSON body, returns `400 Bad Request` and error text.
  - Request body: `not-json`.
  - Assert: `status=400`.

- [MODEL_NOT_FOUND] With `model` set to a non‑existent id, returns `404 Not Found`.
  - Request JSON: `{ "model": "does-not-exist.gguf", "prompt": "hi" }`.
  - Assert: `status=404`.

- [NO_DEFAULT_AND_NO_MODEL] If no `--default-model` is provided and request omits `model`, returns `404 Not Found`.
  - Server started without `--default-model`.
  - Request JSON: `{ "prompt": "hi" }`.
  - Assert: `status=404`.

- [HAPPY_SWITCH_EXPLICIT_MODEL] With ≥2 models available, send two requests to exercise switching.
  - Precondition: registry has at least two models (e.g., `alpha.gguf`, `beta.gguf`).
  - Step 1: `POST /infer` with `{ "model": "alpha.gguf", "prompt": "A" }` → `200` and NDJSON.
  - Step 2: `POST /infer` with `{ "model": "beta.gguf", "prompt": "B" }` → `200` and NDJSON.
  - Assert: both responses return `200`, `Content-Type` includes `application/x-ndjson`, body contains multiple newline-delimited JSON objects.
  - Assert: subsequent `GET /status` shows at least one instance for each of `alpha.gguf` and `beta.gguf` (≥2 total), or otherwise reflects the most recent model ready depending on budgeting.

- [HAPPY_DEFAULT_THEN_EXPLICIT] Start with default-only infer, then switch explicitly.
  - Precondition: `--default-model` set to `alpha.gguf`; `beta.gguf` also exists.
  - Step 1: `POST /infer` with `{ "prompt": "hello" }` (no model) → uses default `alpha.gguf`.
  - Step 2: `POST /infer` with `{ "model": "beta.gguf", "prompt": "hi" }`.
  - Assert: both `200` with NDJSON; `GET /readyz` becomes `200` after the first infer if not already.

- [HAPPY_REPEAT_INFER_SAME_MODEL] Multiple inference requests on the same model are independently successful and return NDJSON.
  - Steps: send two sequential `POST /infer` calls for `alpha.gguf`.
  - Assert: both `200`, NDJSON lines; `GET /status` maintains or updates `LastUsed` for the instance (if exposed); readiness remains `200`.

- [HAPPY_CONTENT_TYPE_AND_STREAMING] Validate streaming and headers more strictly.
  - Assert: `Content-Type` is exactly `application/x-ndjson` (or contains it), response body contains at least two lines, last line contains a terminal marker like `{ "done": true }` (per current stub), and no single line is empty.

- [HAPPY_READY_AFTER_SWITCH] After switching models with an explicit infer, readiness remains `200`.
  - Steps: infer default; poll `readyz` to `200`; infer explicit other model; verify `readyz` still `200`.

- [HAPPY_MODELS_LIST_CONTAINS_DEFAULT] `/models` response includes the configured default model id.
  - Precondition: server started with `--default-model` set to an existing id.
  - Assert: the `models` array contains an entry with that id.

- [TOO_BUSY_429] Under configured backpressure conditions, returns `429 Too Many Requests`.
  - Precondition: configure manager to a tiny per‑instance queue (see `ManagerConfig` via constructor, or run two concurrent `/infer` requests to the same model when only one in‑flight is allowed). In current implementation, backpressure mapping is exposed when `tooBusyError` is returned inside `manager`.
  - Action: start one long‑running `/infer` (e.g., hold the connection or simulate delay), then immediately send another `/infer` for the same model.
  - Assert: second request receives `status=429`.

- [CLIENT_CANCELLED] If the client cancels the request context mid‑stream, server should stop without producing a 500.
  - Action: open `/infer` with streaming enabled, read at least one NDJSON line, then abort the HTTP request by closing the response/connection.
  - Assert: the server does not produce a `500` for that request and continues to serve subsequent requests (e.g., a fresh `/infer` returns `200` and streams NDJSON).
  - Implemented in `tests/e2e_py/test_blackbox.py` as `test_blackbox_client_cancellation_mid_stream`.

- [SERVER_SHUTDOWN_CANCELS] On graceful shutdown, in‑flight `/infer` requests are canceled promptly.
  - Action: start `/infer` (streaming), then send `SIGTERM`/`SIGINT` to the server process.
  - Assert: server exits cleanly and does not emit `500` responses; any new connections are refused after shutdown begins. (Note: explicit e2e test optional; handler cancellation is wired via a process‑level base context.)

## Process‑Level E2E Flow (Black‑Box)

1) Build binary: `go build -o bin/modeld ./cmd/modeld`.
2) Create temp models dir and touch `alpha.gguf`, `beta.gguf`.
3) Start process (child) with:
   - `--addr :18080`
   - `--models-dir <tempdir>`
   - `--default-model alpha.gguf`
4) Wait for `GET /healthz` to be `200`.
5) Verify `GET /models` returns 2 models.
6) Verify initial `GET /readyz` is `503`.
7) Call `POST /infer` without `model`; expect `200` and streaming NDJSON.
8) Poll `GET /readyz` until `200` (or timeout).
9) Verify `GET /status` shows at least one instance.
10) Negative cases:
    - `POST /infer` with `model=missing.gguf` → `404`.
    - Start server without `--default-model`, `POST /infer` with no `model` → `404`.
    - Backpressure scenario to elicit `429` (advanced; see note above).
11) Stop process and cleanup temp files.

## Notes and Current Behaviors (as of codebase)

- `internal/httpapi/server.go` maps errors:
  - Invalid JSON → `400`.
  - `HTTPError` implementations determine status (manager uses specific error types).
  - Otherwise → `500`.
- `internal/manager/infer.go` currently emits small NDJSON token chunks with newlines and a terminal `{ "done": true }` entry.
- `internal/registry.GGUFScanner` discovers models by `*.gguf` suffix; files need not be non‑empty for discovery.

## Future Extensions

- Contract tests against an OpenAPI spec (once available) to validate schema.
- SSE/WebSocket event tests (when implemented).
- Metrics/tracing observability checks.
- Auth/RBAC tests (once added).
- Performance and soak tests (k6) separate from functional E2E.
