# CHANGE_REQUIREMENTS

This document aggregates refinement opportunities and change requirements beyond the MVP for `modeld-go`.

Each item includes rationale, scope, target files, and acceptance criteria. Use this to plan PRs and track progress.

---

## 1) Observability and Logging

- Request-level logging controls
  - Rationale: Per-request overrides exist (`X-Log-Level`, `?log=`) but not documented/configurable broadly.
  - Scope: Document header/query overrides and the default log level flag/config already present.
  - Targets: `README.md`
  - Acceptance: README documents header (`X-Log-Level`) and query (`?log=`) overrides.

## 2) Metrics (Prometheus)

Done: `/metrics` endpoint is implemented via `promhttp` in `internal/httpapi/server.go`, with middleware in `internal/httpapi/metrics.go`.

Exposed metrics (namespace `modeld`, subsystem `http`):
- `modeld_http_requests_total{path,method,status}` (counter)
- `modeld_http_request_duration_seconds{path,method,status}` (histogram)
- `modeld_http_inflight_requests{path}` (gauge)
- `modeld_http_backpressure_total{reason}` (counter)

Note: Path labels currently use the request path; consider mapping to stable route names to reduce cardinality.

## 3) API Hardening and Versioning

Deferred for now to keep scope focused and velocity high.

## 4) Request Validation and Controls

- Enrich `InferRequest` with generation parameters
  - Rationale: Typical inference controls missing (max tokens, temperature, etc.).
  - Scope: Extend `pkg/types/api.go` with fields like `max_tokens`, `temperature`, `top_p`, `stop`, `seed`; validate.
  - Targets: `pkg/types/api.go`, `internal/httpapi/server.go` (validation), tests.
  - Acceptance: 400s on invalid values; fields pass through to manager (placeholder for now).

- Stable streaming schema
  - Rationale: Define NDJSON contract; always emit terminal line.
  - Scope: Ensure final line is `{ "done": true, "metrics": { ... } }` and document.
  - Targets: `internal/httpapi/server.go`, README, tests.
  - Acceptance: Tests assert terminal done line presence.

## 5) Backpressure and Timeouts
 
Done: Backpressure knobs (`--max-queue-depth`, `--max-wait`) and `/infer` timeout (`--infer-timeout`) are implemented and wired through config/flags.

## 6) Security (Opt-in)

Deferred; consider in a future phase if deployment context requires it.

## 7) Readiness, Warmup, and Status Enrichment

- Pre-warm default model
  - Rationale: Faster first-token latency; quicker `/readyz` flip.
  - Scope: `--prewarm-default` flag to pre-load and warm the configured default model.
  - Targets: `cmd/modeld/main.go`, `internal/manager/*`.
  - Acceptance: With flag set, `/readyz` becomes 200 without a prior `/infer`.

- Status enrichment
  - Rationale: Improve operability.
  - Scope: Extend `types.InstanceStatus` and `StatusResponse` with `last_error`, `uptime_seconds`, `server_time_unix`, `evictions_total`, `loads_total`.
  - Targets: `pkg/types/api.go`, `internal/manager/status.go`.
  - Acceptance: `/status` returns new fields; tests updated.

## 8) Middlewares and Headers

Done: Configurable CORS is implemented (flags and config; preflight behavior covered by tests).


## 9) CI/CD and Tooling

- Race detector and linters
  - Rationale: Catch data races and common issues.
  - Scope: Add `go test -race` and `golangci-lint` (or `staticcheck`).
  - Targets: `.github/workflows/ci.yml`, `Makefile`.
  - Acceptance: CI runs race tests and lint; thresholds enforced.

- Dockerfile
  - Rationale: Containerized distribution and deployment.
  - Scope: Multi-stage build; non-root; minimal base; read-only FS.
  - Targets: `Dockerfile` (new), `README.md`.
  - Acceptance: `docker build` succeeds; image runs and passes healthz.

## 10) Developer Experience

- Example configs
  - Rationale: `scripts/dev-run.sh` references `configs/models.yaml` which is not in repo.
  - Scope: Add `configs/models.example.yaml` and update script/docs.
  - Targets: `configs/models.example.yaml` (new), `scripts/dev-run.sh`, `README.md`.
  - Acceptance: `make run` and script instructions work out-of-the-box.

- Make targets
  - Rationale: Convenience.
  - Scope: Add `make lint`, `make run-metrics`, `make docker`.
  - Targets: `Makefile`.
  - Acceptance: Targets execute successfully.

## 11) Testing Enhancements

- Unit tests coverage expansion
  - Rationale: Improve confidence; CI requires ≥80%.
  - Scope: Add tests for `internal/config/loader.go` (YAML/JSON/TOML), flag precedence merge; `requestLogLevel()` and `loggingLineWriter`.
  - Targets: `internal/config/loader_test.go`, `internal/httpapi/server_test.go`.
  - Acceptance: New tests pass and increase coverage.

- Fault injection and cancellation tests
  - Rationale: Validate resilience.
  - Scope: Manager stubs to simulate slow generations, partial writes, and errors; verify correct mapping and cancellation.
  - Targets: `internal/manager/*_test.go`, `internal/httpapi/server_test.go`.
  - Acceptance: Deterministic tests covering 404/429/500, client-cancel, shutdown.

<!-- Optional load and rate tests are omitted for now to keep scope minimal. -->

## 12) Security and Privacy

Deferred; current defaults avoid logging full prompts unless request-level debug is enabled.

---

## Proposed PR Sequence

1. Reliability and readiness
   - Optional prewarm default model.
2. Request model improvements
   - Enrich `InferRequest` and add validation.
3. CI/CD and DX
   - Race tests, linting, Dockerfile, example configs.

---

## Cross-References (Current Code Pointers)

- HTTP server and routes: `internal/httpapi/server.go`
- HTTP tests: `internal/httpapi/server_test.go`
- E2E tests (Go): `internal/e2e/e2e_test.go`
- Blackbox tests (Python): `tests/e2e_py/test_blackbox.py`
- Types: `pkg/types/api.go`, `pkg/types/domain.go`
- Manager status: `internal/manager/status.go`
- Config loader: `internal/config/loader.go`
- Entrypoint: `cmd/modeld/main.go`
- CI workflow: `.github/workflows/ci.yml`
- Makefile: `Makefile`

---

## Notes

- When expanding `InferRequest`, ensure tests assert validation and the terminal `{"done": true}` line.
- Avoid logging secrets or full prompts by default. Provide per-request override via headers for debugging sessions.
