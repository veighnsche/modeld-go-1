# Testing

This repository has multiple test suites and a helper CLI to orchestrate them.

## Go tests

- Run all Go tests:
  ```bash
  make test
  ```
- Coverage subset + HTML:
  ```bash
  make cover
  make cover-html
  ```
- Enforce minimum coverage (80%):
  ```bash
  make cover-check
  ```

## Python black-box tests

- Location: `tests/e2e_py/`
- One-liner:
  ```bash
  make e2e-py
  ```
- What it does:
  - Creates a venv
  - Installs from `tests/e2e_py/requirements.txt`
  - Builds and runs the server as a subprocess on a free port
  - Runs pytest against the live HTTP API

## Web harness + Cypress E2E

- Harness location: `web/` (Vite + React)
- Cypress specs: `e2e/specs/*.cy.ts`
- Orchestration (root `package.json`):
  - `dev:api` — `make run`
  - `dev:web` — `pnpm -C web dev`
  - `dev:all` — runs both concurrently
  - `test:e2e:open` — waits for services then opens Cypress
  - `test:e2e:run` — waits then runs Cypress headless

Notes:
- `scripts/cli/poll-url.js` is used before Cypress to avoid race conditions.
- Install once at repo root with pnpm workspaces:
  ```bash
  pnpm install
  ```

## testctl (Go CLI helper)

A thin CLI that installs tools and runs tests across Go, Python, and Cypress.

- Build:
  ```bash
  make testctl-build
  ```
- Install helpers:
  ```bash
  bin/testctl install all    # JS, Go, Python
  bin/testctl install js
  bin/testctl install go
  bin/testctl install py
  bin/testctl install host:act   # install `act` for local CI runs
  ```
- Run tests:
  ```bash
  bin/testctl test go
  bin/testctl test api:py
  bin/testctl test web host
  bin/testctl test web auto
  bin/testctl test all auto
  ```

The UI suite enforces a strict rule: UI tests require host models in `~/models/llm`. The `test web auto` command will run against the API when host models exist; otherwise it will error.
