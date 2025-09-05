# Build and Run

Requirements: Go 1.22+

## Build

- Build the main binary:
  ```bash
  make build
  ```
  Outputs: `bin/modeld`

- Tidy modules:
  ```bash
  make tidy
  ```

## Run

- Run from source:
  ```bash
  make run
  ```
- Or run the built binary:
  ```bash
  bin/modeld [flags]
  ```

### Flags (from `cmd/modeld/main.go`)

- `--addr` (env: `MODELD_ADDR`), default `:8080`
- `--config` path to YAML/JSON/TOML config file (optional)
- `--models-dir` directory to scan for `*.gguf` (default `~/models/llm`)
- `--vram-budget-mb` integer VRAM budget across all instances (0 = unlimited)
- `--vram-margin-mb` integer VRAM margin to keep free
- `--default-model` default model id when omitted in requests

Notes:
- Client disconnects during `POST /infer` will cancel the in-flight generation.
- Graceful shutdown cancels in-flight and queued requests.

## Configuration: Manager modes and fields

The manager supports two inference modes selected via `ManagerConfig` (and corresponding CLI/env in `cmd/modeld`):

- Server mode (external llama.cpp server): set `LlamaServerURL` to the base URL of a running server. The adapter will use OpenAI-compatible endpoints and all HTTP requests use context-based timeouts.
- Spawn mode (managed local llama.cpp): set `SpawnLlama=true` and provide `LlamaBin` (path to `llama-server`). A subprocess is spawned per model with:
  - `LlamaHost` (default `127.0.0.1`)
  - `LlamaPortStart`, `LlamaPortEnd` (optional port range; 0 means auto)
  - `LlamaThreads`, `LlamaCtxSize`, `LlamaNGL`, and `LlamaExtraArgs` for common flags

Precedence: Spawn mode takes precedence when enabled (`SpawnLlama=true` and `LlamaBin` set). Otherwise, if `LlamaServerURL` is set, server mode is used. If neither is configured, inference endpoints will return a dependency-unavailable error (503).

## Swagger (OpenAPI) Docs

This project includes Swagger annotations and can serve a Swagger UI when built with the `swagger` build tag.

- Generate docs (outputs to `docs/`):
  ```bash
  make swagger-gen
  ```
- Run with Swagger UI enabled:
  ```bash
  make swagger-run
  # Open http://localhost:8080/swagger/index.html
  ```
- Build a swagger-enabled binary:
  ```bash
  make swagger-build
  ```

Notes:
- Default builds do not include the Swagger UI routes. The `internal/httpapi/MountSwagger()` no-op is replaced by a UI mount when using `-tags=swagger`.
- The Makefile pins `swag` to a specific version for reproducible docs; CI also regenerates and verifies docs are up to date.
