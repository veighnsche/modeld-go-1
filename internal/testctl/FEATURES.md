# testctl package — Features Overview

This document summarizes the capabilities provided by the `internal/testctl/` package.

## CLI entrypoints and configuration

- __Commands__ (`internal/testctl/cli.go`)
  - `install` — environment setup
    - `all` — runs `js`, `go`, `py`
    - `js` — install JavaScript dependencies via `pnpm`
    - `go` — download Go modules
    - `py` — set up Python venv and install `tests/e2e_py/requirements.txt`
    - `llama`, `llama:cuda`, `go-llama.cpp`, `go-llama.cpp:cuda` — install/build llama.cpp with CUDA
    - `host:docker` — install Docker (Arch Linux focused)
    - `host:act` — install `act` (GitHub Actions local runner)
    - `host:all` — `host:docker` + `host:act`
  - `test` — test runners
    - `go` — run all Go tests
    - `api:py` — run Python API E2E tests
    - `py:haiku` — run only the Python haiku E2E test
    - `web <mode>` — run Cypress UI tests
      - `mock` — run UI against mocked API
      - `live:host` — run UI against local server using host models
      - `haiku` — run only the Haiku Cypress spec against live backend (no mocks)
      - `auto` — chooses `live:host` if host models exist, else `mock`
    - `all auto` — run Go tests, Python tests, then UI tests (auto mode)
  - `test ci` — run GitHub Actions locally via `act`
    - `all [runner:catthehacker|runner:default] [-- <extra act args>]`
    - `one <workflow.yml|yaml> [runner:catthehacker|runner:default] [-- <extra act args>]`
- __Flags and Env__ (`internal/testctl/cli.go`, `internal/testctl/logenv.go`)
  - Flags: `--web-port` (default from `WEB_PORT` or `5173`), `--log-level` (`debug|info|warn|error`, default from `TESTCTL_LOG_LEVEL` or `info`)
  - Environment helpers: `envStr`, `envInt`, `envBool`
  - Logging levels: `SetLogLevel`, `debug/info/warn/errl` with RFC3339 timestamps

## Installers

- __JavaScript__ (`internal/testctl/install_js.go`)
  - Ensures `pnpm` is available (attempts `corepack enable`/`prepare`), then runs `pnpm install` in repo root and `web/` with `--frozen-lockfile`.
- __Go__ (`internal/testctl/install.go`)
  - `go mod download`
- __Python__ (`internal/testctl/install.go`)
  - Creates `tests/e2e_py/.venv`, runs `python3 -m venv` and installs `requirements.txt`.
- __Host: Docker__ (Arch Linux focused) (`internal/testctl/install_host.go`)
  - Installs `docker` via `pacman` if missing, enables service, adds current user to `docker` group, smoke checks `docker --version`.
- __Host: act__ (local GitHub Actions runner) (`internal/testctl/install_host.go`)
  - Prefers AUR via `yay` or `paru` (`act-bin` or `act`), else falls back to downloading and extracting the official tarball to `/usr/local/bin`.
- __Llama.cpp with CUDA__ (`internal/testctl/install_llama.go`)
  - Installs prerequisites on Arch (base-devel, cmake, ninja, cuda, etc.).
  - Clones/updates `~/src/llama.cpp`, configures CMake with CUDA, prefers `g++-14` as host compiler if available.
  - Builds `libllama.so` in `build-cuda14` and prints next-step env exports (`LLAMA_CPP_DIR`, `CGO_CFLAGS`, `CGO_LDFLAGS`, `LD_LIBRARY_PATH`, `CGO_ENABLED`).

## Test runners

- __Go tests__ (`internal/testctl/tests.go`)
  - `go test ./... -v`
- __Python E2E tests__ (`internal/testctl/tests.go`)
  - Uses venv `tests/e2e_py/.venv/bin/pytest`, auto-installs venv if missing.
- __Python Haiku test__ (`internal/testctl/tests.go`)
  - Runs only `tests/e2e_py/test_haiku.py` with `-s` to print the poem in logs.

## Web UI E2E suites (Cypress)

- __Shared helpers__ (`internal/testctl/web_helpers.go`)
  - `buildWebWith(env)` — builds `web/` with provided env.
  - `startPreview(port)` — starts Vite preview on given port; tracked for cleanup.
  - `runCypress(env, args...)` — runs default `pnpm run test:e2e:run` or a custom invocation (e.g., `xvfb-run ... cypress run`).
  - `findLlamaBin()` — searches common locations/`PATH` for `llama-server`.
- __Mock mode__ (`internal/testctl/web_mock.go`)
  - Picks a free web port, builds with `VITE_USE_MOCKS=1`, starts preview, waits for readiness, runs Cypress with `CYPRESS_BASE_URL` and `CYPRESS_USE_MOCKS=1`.
- __Live: Host mode__ (`internal/testctl/web_live.go`)
  - Requires host models (`~/models/llm/*.gguf`).
  - Chooses API port (prefers `18080`) and web port (prefers `--web-port`).
  - Starts `go run ./cmd/modeld` with `--models-dir` and `--default-model`, enables CORS, waits for `/healthz`.
  - Builds web with `VITE_USE_MOCKS=0`, `VITE_API_BASE_URL`, `VITE_SEND_STREAM_FIELD=true`, starts preview, runs Cypress.
- __Haiku Live: Host mode (no mocks)__ (`internal/testctl/web_haiku.go`)
  - Same as Live:Host, but enables `--real-infer` and passes `--llama-bin` (via `findLlamaBin()`).
  - Runs only `e2e/specs/haiku_center.cy.ts` via a custom Cypress command (headless with `xvfb-run`).

## Filesystem, ports, and processes utilities

- __Filesystem helpers__ (`internal/testctl/fs.go`)
  - `firstGGUF(dir)` — returns first `*.gguf` model filename in a directory.
  - `hasHostModels()` — detects presence of `~/models/llm/*.gguf`.
  - `homeDir()` — resolves home directory from `HOME` or OS.
- __Ports & HTTP readiness__ (`internal/testctl/ports.go`)
  - `chooseFreePort()` — asks kernel for a free TCP port.
  - `isPortBusy(port)` — quick TCP connect check.
  - `waitHTTP(url, want, timeout)` — poll until HTTP returns desired status code.
  - `ensurePorts(ports, force)` — verify/optionally free ports via `fuser -k`.
  - `preferOrFree(desired)` — use desired if free, else allocate a free port.
- __Process management__ (`internal/testctl/process.go`)
  - `ProcManager` to track and kill spawned processes.
  - Package-level helpers: `TrackProcess(cmd)`, `killProcesses()` (delegates to manager).
- __Command execution__ (`internal/testctl/executil.go`)
  - `RunCmd(ctx, Cmd)` — unified runner supporting env/dir and streaming.
  - Helpers: `runCmdVerbose`, `runCmdStreaming`, `runEnvCmdStreaming`.

## CI integration via act

- __Workflow discovery__ (`internal/testctl/ci.go`)
  - `listWorkflows()` — lists `.yml|.yaml` under `.github/workflows/`.
- __Run single workflow__ (`internal/testctl/ci.go`)
  - `runCIWorkflow(workflowFile, useCatthehacker, extraArgs)` — adds `-P` mappings to catthehacker images when requested.
- __Run all workflows__ (`internal/testctl/ci.go`)
  - `runCIAll(useCatthehacker, extraArgs)` — iterates all discovered workflows.
- __CLI conveniences__ (`internal/testctl/cli.go`)
  - Supports `runner:catthehacker` (default) or `runner:default` token.
  - Supports passing extra `act` arguments after `--`.

## OS detection and platform assumptions

- __Arch-like detection__ (`internal/testctl/os_helpers.go`)
  - `isArchLike()` — inspects `/etc/os-release` for `ID`/`ID_LIKE` containing `arch`.
- __Platform focus__
  - Host installers target Arch Linux; on other OSes, users are prompted to install manually where applicable.

## Testability and indirection

- __Action indirection for stubbing__ (`internal/testctl/actions.go`)
  - Exposes package-level function variables (e.g., `fnInstallJS`, `fnRunGoTests`, `fnTestWebMock`, `fnRunCIAll`, etc.) to allow tests to stub behavior without invoking real side effects.

## Notable environment variables used at runtime

- __Web build__
  - `VITE_USE_MOCKS` — `1` for mock, `0` for live
  - `VITE_API_BASE_URL` — backend API base URL
  - `VITE_SEND_STREAM_FIELD` — enable stream field in requests
- __Cypress__
  - `CYPRESS_BASE_URL` — target UI URL
  - `CYPRESS_USE_MOCKS` — signals mock mode to specs
  - `CYPRESS_API_READY_URL`, `CYPRESS_API_STATUS_URL` — used by Haiku live flow
- __General__
  - `WEB_PORT` — default for `--web-port`
  - `TESTCTL_LOG_LEVEL` — default for `--log-level`
