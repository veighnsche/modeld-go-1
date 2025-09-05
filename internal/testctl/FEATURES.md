# testctl — Audience-Oriented Feature Overview

This document summarizes `internal/testctl/` capabilities, organized by environment:

- Go Production
- Go Testing
- Go E2E Testing
- Python E2E Testing
- Cypress E2E Testing
- CI Testing

It focuses on what you can do, with concise pointers to implementation files.

## Quick map of CLI (internal/testctl/cli.go)

- install
  - all — [Dev]
  - nodejs — [Cypress]
  - go — [Go Test]
  - py — [Py E2E]
  - llama | llama:cuda | go-llama.cpp | go-llama.cpp:cuda — [Go Prod]
  - host:act — [CI]
  - host:docker — [CI]
  - host:all — [CI]
- test
  - go — [Go Test] (+ includes [Go E2E])
  - api:py — [Py E2E]
  - py:haiku — [Py E2E]
  - web <host|haiku|auto> — [Cypress]
  - all auto — [Orchestration] (Go → Py → Web)
- test ci
  - all [runner:catthehacker|runner:default] [-- <extra act args>] — [CI]
  - one <workflow.yml|yaml> [runner:…] [-- <extra act args>] — [CI]

__Flags & logging__ (`cli.go`, `logenv.go`)
- --web-port (defaults to `WEB_PORT` or `5173`)
- --log-level `debug|info|warn|error` (defaults to `TESTCTL_LOG_LEVEL` or `info`)
- Logging helpers with RFC3339 timestamps; env helpers `envStr/envInt/envBool`

Note: `test web ...` subcommands are exclusively for Cypress E2E testing (not a general dev/prod server).
Note: `install host:docker` and `install host:all` are CI-focused installers (not needed for general dev/prod).

__Legend__
- [Go Prod] Go Production
- [Go Test] Go Testing
- [Go E2E] Go E2E Testing
- [Py E2E] Python E2E Testing
- [Cypress] Cypress E2E Testing
- [CI] CI Testing (via `act` and host tools)
- [Dev] Development utilities
- [Orchestration] Convenience combined runner

---

## Go Production

Focus: preparing model backends and host models for running the Go service with real inference.

### Install

- Llama.cpp (CUDA variants supported) — `install llama | llama:cuda | go-llama.cpp | go-llama.cpp:cuda`
  - Implementation: `internal/testctl/install_llama.go`
  - Notes: Linux-focused prerequisites (tested on Arch, Debian, and Ubuntu); builds `libllama.so`; prints env exports (`LLAMA_CPP_DIR`, `CGO_CFLAGS`, `CGO_LDFLAGS`, `LD_LIBRARY_PATH`, `CGO_ENABLED`).
- Host models
  - Expected at `~/models/llm/*.gguf` (detected by `hasHostModels()` in `fs.go`).

### Environment

- `TESTCTL_LOG_LEVEL` — logging verbosity

### CLI: commands & options

- `install llama`
- `install llama:cuda`
- `install go-llama.cpp`
- `install go-llama.cpp:cuda`
  - No additional parameters; see printed env exports after build.
- Host models expected at `~/models/llm/*.gguf`

### Refs

- Installers: `internal/testctl/install_llama.go`
- FS helpers: `internal/testctl/fs.go`

---

## Go Testing

Focus: unit and integration tests for Go code.

- Run — `test go` → `go test ./... -v` (`internal/testctl/tests.go`)
- Coverage and verbosity flags can be passed via standard `go test` args.

### CLI: commands & options

- `test go`
  - Runs `go test ./... -v`
  - No custom flags via `testctl`; use `go test` locally as needed for extra options.

### Refs

- Runner: `internal/testctl/tests.go`

## Go E2E Testing

Focus: end-to-end Go tests under `internal/e2e/` (executed by the same `test go`).

- Included in `test go`.
- Utilities available: ports, processes, fs helpers
  - `ports.go`: `chooseFreePort`, `waitHTTP`, `preferOrFree`
  - `process.go`: `ProcManager`, `TrackProcess`, `killProcesses`
  - `fs.go`: `firstGGUF`, `hasHostModels`

### CLI: commands & options

- `test go`
  - Executes E2E tests under `internal/e2e/` as part of the full Go test suite.

### Refs

- Suites: `internal/e2e/*.go`
- Utilities: `internal/testctl/ports.go`, `internal/testctl/process.go`, `internal/testctl/fs.go`

## Python E2E Testing

Focus: exercising the HTTP API from Python.

- Install — `install py` (creates venv at `tests/e2e_py/.venv`, installs `requirements.txt`) (`install.go`)
- Run all — `test api:py` (auto-creates venv if missing) (`tests.go`)
- Run haiku-only — `test py:haiku` (`tests.go`)

### CLI: commands & options

- `install py`
  - Creates venv at `tests/e2e_py/.venv`, installs `requirements.txt`.
- `test api:py`
  - Runs all Python E2E tests using the venv; auto-creates venv if missing.
- `test py:haiku`
  - Runs only the haiku test and prints poem output (`-s`).

### Refs

- Installer: `internal/testctl/install.go`
- Runner: `internal/testctl/tests.go`
- Tests: `tests/e2e_py/`

## Cypress E2E Testing (Live-only)

Focus: UI end-to-end testing. Web commands are Cypress-only and not a general dev/prod server.

- Install (JS deps) — `install nodejs` (ensures pnpm; installs root and `web/` with frozen lockfiles) (`install_nodejs.go`)
- Run — `test web <host|haiku|auto>`
  - host: requires host `*.gguf` models; starts API; runs Cypress (`web_live.go`)
  - haiku: host; runs haiku spec only (`web_haiku.go`)
  - auto: runs `host` when host models exist; otherwise errors (no mock fallback)
  - Env vars: `VITE_API_BASE_URL`, `VITE_SEND_STREAM_FIELD`, `CYPRESS_BASE_URL`, `CYPRESS_API_READY_URL`, `CYPRESS_API_STATUS_URL`

### CLI: commands & options

- `install nodejs`
  - Ensures `pnpm` via `corepack`; installs root and `web/` deps with `--frozen-lockfile`.
- `test web host`
  - Requires host `*.gguf` models; starts local API; builds UI; runs Cypress.
- `test web haiku`
  - Like `live:host` but runs only the haiku spec with real infer.
- `test web auto`
  - Chooses `host` if host models exist; otherwise errors.
- Options
  - `--web-port <port>` (defaults to `WEB_PORT` or `5173`) — used for the Vite preview.

### Refs

- Installer: `internal/testctl/install_nodejs.go`
- Web helpers: `internal/testctl/web_helpers.go`
- Modes: `internal/testctl/web_live.go`, `internal/testctl/web_haiku.go`

## CI Testing

Focus: running GitHub Actions locally and preparing a CI-like host.

- Install CI tools
  - GitHub Actions local runner — `install host:act` (`install_host.go`) — supports Arch, Debian, and Ubuntu (apt or GitHub release fallback)
  - Docker for CI — `install host:docker` (CI-only) (`install_host.go`) — supports Arch (pacman) and Debian/Ubuntu (apt)
  - CI combo — `install host:all` (docker + act; CI-only) (`install_host.go`)
- Run workflows with `act` (`ci.go`)
  - All — `test ci all [runner:catthehacker|runner:default] [-- <extra act args>]`
  - One — `test ci one <workflow.yml|yaml> [runner:…] [-- <extra act args>]`
  - Discovery — workflows under `.github/workflows/` (`listWorkflows()`)

### CLI: commands & options

- `install host:act` — install GitHub Actions local runner
- `install host:docker` — install Docker (CI-only)
- `install host:all` — docker + act (CI-only)
- `test ci all [runner:catthehacker|runner:default] [-- <extra act args>]`
  - Runner token selects image mappings; extra args forwarded to `act`.
- `test ci one <workflow.yml|yaml> [runner:catthehacker|runner:default] [-- <extra act args>]`
  - Executes a single workflow file from `.github/workflows/`.

### Refs

- CLI: `internal/testctl/cli.go`
- Act integration: `internal/testctl/ci.go`
- Workflows: `.github/workflows/*.yml|yaml`

## Development Utilities

Focus: local iteration helpers.

- One-shot setup — `install all` (runs `nodejs`, `go`, `py`)
- Web helpers (`web_helpers.go`): `buildWebWith`, `startPreview`, `runCypress`, `findLlamaBin`
- Exec (`executil.go`): `RunCmd`, `runCmdVerbose`, `runCmdStreaming`, `runEnvCmdStreaming`
- Testability indirection (`actions.go`): `fnInstallJS`, `fnRunGoTests`, `fnTestWebMock`, `fnRunCIAll`
  - OS detection (`os_helpers.go`): `isArchLike()`, `isDebianLike()`, `isUbuntuLike()` (installers support Arch and Debian/Ubuntu)

### CLI: commands & options

- `install all` — runs `install nodejs`, `install go`, `install py`
- Logging level for all commands: `--log-level debug|info|warn|error` (or `TESTCTL_LOG_LEVEL`)

### Refs

- CLI and flags: `internal/testctl/cli.go`, `internal/testctl/logenv.go`
- Execution: `internal/testctl/executil.go`
- Test indirection: `internal/testctl/actions.go`
- OS detection: `internal/testctl/os_helpers.go`


