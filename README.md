# modeld-go (scaffold)

![CI](https://github.com/veighnsche/modeld-go-1/actions/workflows/ci-go.yml/badge.svg)
[![codecov](https://codecov.io/gh/veighnsche/modeld-go-1/graph/badge.svg)](https://codecov.io/gh/veighnsche/modeld-go-1)

Minimal scaffold for a Go control-plane around llama.cpp:
- Model switching
- Readiness + events (SSE/WebSocket later)
- Clean HTTP API surface

---

# modeld-go

A lightweight control-plane service (Go 1.22+) to manage multiple preloaded llama.cpp model instances within a configurable VRAM budget and margin, and to serve inference requests over a clean HTTP API. Hot math stays in llama.cpp (C/C++); Go handles lifecycle, I/O, backpressure, and observability.

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

## Visual Harness + Cypress E2E (Testing Add-on)

This repo includes a minimal React-based visual test harness and a Cypress E2E suite to validate end-to-end behavior of the HTTP API (UI → API → stream → UI). This harness is strictly for testing and is not a production UI.

### Structure

- `web/` — Vite + React Router multi-page harness
  - Pages:
    - `Infer` — original streaming infer UI and controls
    - `Health` — fetches and shows `/healthz`
    - `Ready` — fetches and shows `/readyz`
    - `Models` — fetches and shows `/models` (supports array or `{ models: [...] }`)
    - `Status` — fetches and shows `/status`
  - Top-level navigation links between pages
  - Uses environment variables (no hardcoded URLs) and exposes a set of `data-testid` elements for automation
- `e2e/` — Cypress E2E tests (e2e mode)
  - Config: `e2e/cypress.config.ts`
  - Specs: `e2e/specs/*.cy.ts`
  - Support: `e2e/support/`
- `scripts/cli/poll-url.js` — small Node helper to poll a URL until it returns an expected status
- `package.json` (root) — orchestration scripts (with placeholders to adapt to your environment)

### Web Harness

Environment variables (via `web/.env` or your shell):

- `VITE_API_BASE_URL` (e.g. `http://localhost:8080`)
- `VITE_HEALTH_PATH` (default `/healthz`)
- `VITE_READY_PATH` (default `/readyz`)
- `VITE_MODELS_PATH` (default `/models`)
- `VITE_STATUS_PATH` (default `/status`)
- `VITE_INFER_PATH` (default `/infer`)
- `VITE_SEND_STREAM_FIELD` (default `false`) — include `{ "stream": true }` in POST if your server requires it
- `VITE_USE_MOCKS` (`true|false`, default `false`) — mock mode bypasses network and emits a deterministic NDJSON sequence

Rendered test IDs in the SPA:

- `data-testid="mode"` — `mock|live`
- `data-testid="models-count"` — optional models count (calls `/models` on load)
- `data-testid="prompt-input"` — textarea for prompt
- `data-testid="model-input"` — optional model id (blank to use default)
- `data-testid="submit-btn"` — send request
- `data-testid="status"` — `idle|requesting|success|error`
- `data-testid="stream-log"` — NDJSON lines appended as they arrive
- `data-testid="result-json"` — full last response (best-effort) as JSON string
- `data-testid="latency-ms"` — measured end-to-end duration in milliseconds

### Cypress E2E

Env file (copyable example at `.env.test.example`):

- `CYPRESS_BASE_URL` — the harness URL (e.g., `http://localhost:5173` if using Vite preview default)
- `CYPRESS_API_HEALTH_URL` — API health URL (e.g., `http://localhost:8080/healthz`)
- `CYPRESS_API_READY_URL` — API ready URL (e.g., `http://localhost:8080/readyz`)
- `CYPRESS_API_STATUS_URL` — API status URL (e.g., `http://localhost:8080/status`)
- `CYPRESS_MAX_LATENCY_MS` — threshold for end-to-end latency (default 5000)
- `CYPRESS_USE_MOCKS` — when `"1"`/`true`, E2E skips live API checks and relies on mock streaming

Notes:
- Cypress automatically picks up any environment variables prefixed with `CYPRESS_` and makes them available as `Cypress.env('<NAME_WITHOUT_PREFIX>')`.
- For local runs you may also set `USE_MOCKS` in your shell and map it inside tests, but using the `CYPRESS_` prefix is recommended for CI.

Artifacts on failure are written to `e2e/artifacts/` (screenshots/videos), and the suite includes a task to save arbitrary text files for debugging.

Core spec: `e2e/specs/visual_infer.cy.ts`

- Visits `/`, optionally waits for API `healthz` (live mode)
- Types a prompt, optionally sets a model, clicks send
- Asserts status transitions to `requesting` then `success`
- Asserts `stream-log` contains ≥ 2 lines and final line indicates completion (contains `"done": true`)
- Ensures `result-json` is non-empty and parseable JSON, no console errors, and latency under threshold

Additional specs (best-effort):

- `ready_flow.cy.ts` — verifies `readyz` transitions after first infer (skipped in mock mode)
- `models_list.cy.ts` — verifies `/models` count renders if available
- `errors.cy.ts` — invalid model name surfaces error (expects UI `status=error` and error text)

### Orchestration Scripts (root `package.json`)

The following scripts are provided and ready to use:

- `dev:api` — `make run` (starts the Go API locally)
- `dev:web` — `pnpm -C web dev` (starts Vite dev server)
- `dev:all` — runs both concurrently
- `test:e2e:open` — polls configured URLs then opens Cypress
- `test:e2e:run` — polls configured URLs then runs Cypress headlessly

Notes:

- `scripts/cli/poll-url.js` is used before Cypress to avoid race conditions.
- Install once at repo root with pnpm workspaces: `pnpm install`.
- You can run package scripts in the `web/` workspace via pnpm filtering, e.g. `pnpm -C web build`.

### Go CLI Helper (`bin/testctl`)

A Go-based CLI orchestrates installs and tests across Go, Python, and Cypress. It auto-selects free ports and enforces a strict rule for UI tests: if host models exist in `~/models/llm`, `test web auto` runs against the live API; otherwise it runs in mock mode.

Examples:

```bash
# Build the CLI
make testctl-build

# Install dependencies
bin/testctl install all    # JS, Go, Python
bin/testctl install js
bin/testctl install go
bin/testctl install py

# Run tests
bin/testctl test go                 # go test ./... -v
bin/testctl test api:py             # pytest tests/e2e_py
bin/testctl test web mock           # Cypress UI (mock mode)
bin/testctl test web live:host      # Cypress UI (host models required)
bin/testctl test web auto           # strict auto mode (host models => live)
bin/testctl test all auto           # full suite (Go + Py + UI auto)
```

Notes:

- The CLI will try to enable pnpm via corepack if available; otherwise ensure pnpm is installed.
- Cypress baseUrl is set automatically via `CYPRESS_BASE_URL` to the dynamic preview port.

Shortcuts:

- `pnpm run cli` — runs `bin/testctl`.
- `make test-all` — installs via testctl and runs the full suite with `test all auto`.

#### Implementation structure (testctl)

The CLI is implemented as a thin entrypoint with reusable internal logic:

- `cmd/testctl/main.go` — minimal `main` that delegates to `internal/testctl.Main()`.
- `internal/testctl/` — all CLI logic lives here and is unit-testable:
  - `Run(args []string, cfg *Config) error` — dispatches subcommands.
  - `ParseConfigWith(fs *flag.FlagSet, args []string) (*Config, []string)` — parses flags/env with an injectable `FlagSet` for tests.
  - `ParseConfig()` — small wrapper over `ParseConfigWith` using `flag.CommandLine` and `os.Args[1:]`.
  - `MainWithArgs(args []string) int` — testable entrypoint that returns an exit code.
  - `Main() int` — production entrypoint used by `cmd/testctl`.

Testing the CLI logic (unit):

```bash
go test ./internal/testctl -v
```

These tests stub indirect function variables (e.g., installers, runners) and verify dispatch/flag parsing without spawning subprocesses.

### Quickstart

1. Install deps (pnpm workspaces):
   - `pnpm install`
2. Configure envs:
   - Copy `.env.test.example` → `.env.test` and edit as needed
   - Edit `web/.env` (or use `web/.env.mock` for mock mode)
3. Run everything:
   - `pnpm run dev:all` (fills in your placeholders)
   - Visit the harness at `CYPRESS_BASE_URL`
4. Run tests (load env first):
   - Bash/Zsh: `set -a; source .env.test; set +a`
   - Then run:
     - Headless: `pnpm run test:e2e:run`
     - Interactive: `pnpm run test:e2e:open`

All URLs/ports/paths are env-driven; there are no hardcoded values in the harness or tests.
