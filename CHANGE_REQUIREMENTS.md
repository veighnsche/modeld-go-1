# CHANGE_REQUIREMENTS

This document aggregates refinement opportunities and change requirements beyond the MVP for `modeld-go`.

Each item includes rationale, scope, target files, and acceptance criteria. Use this to plan PRs and track progress.

---

## 1) Observability and Logging

- Structured logging and correlation
  - Rationale: Current logs use `log.Printf` and are unstructured; harder to query and correlate. `middleware.RequestID` exists but is not embedded in logs.
  - Scope:
    - Introduce a structured logger (e.g., `uber-go/zap` or `rs/zerolog`).
    - Include request id, path, model id, status code, and latency in logs.
    - Redact or truncate prompt content to avoid PII leakage.
  - Targets:
    - `internal/httpapi/server.go` (replace `log.Printf` usage)
    - Optionally `cmd/modeld/main.go` (logger setup and wiring)
  - Acceptance:
    - Logs are JSON and include `request_id` on `/infer`, `/status`, `/models`.
    - No full prompts appear in logs unless explicitly `debug`.

- Request-level logging controls
  - Rationale: Per-request overrides exist (`X-Log-Level`, `?log=`) but not documented/configurable broadly.
  - Scope: Expose default log level via flag and config, document overrides.
  - Targets: `internal/httpapi/server.go`, `cmd/modeld/main.go`, `README.md`
  - Acceptance: `--log-level` flag and config key; README documents header/query overrides.

## 2) Metrics (Prometheus)

- Add `/metrics` endpoint
  - Rationale: No current metrics; need visibility into traffic, latency, backpressure, VRAM budgeting.
  - Scope:
    - Integrate `prometheus/client_golang` and expose `promhttp.Handler()` at `/metrics`.
    - Export counters, histograms, and gauges:
      - HTTP request total / duration by `path`, `method`, `status`.
      - In-flight requests per path.
      - Backpressure rejections (429) and reasons.
      - Manager gauges: `budget_mb`, `used_est_mb`, `margin_mb`, per-instance `queue_len`, `inflight`.
  - Targets: `internal/httpapi/server.go` (middleware + route), `internal/manager/status.go` (metrics update points), `go.mod`.
  - Acceptance: `curl /metrics` returns registries and updates during e2e runs.

## 3) API Hardening and Versioning

- Versioned API paths
  - Rationale: Future-proofing breaking changes.
  - Scope: Move application endpoints under `/v1` (e.g., `/v1/models`, `/v1/status`, `/v1/infer`), keep `/healthz` and `/readyz` at root; add temporary aliases for backward compatibility.
  - Targets: `internal/httpapi/server.go`, tests (`internal/httpapi/server_test.go`, `internal/e2e/e2e_test.go`, `tests/e2e_py/test_blackbox.py`), `README.md`.
  - Acceptance: All tests updated and pass using `/v1/*` endpoints.

- OpenAPI specification
  - Rationale: Enable client codegen and documentation.
  - Scope: Author `openapi.yaml` describing all endpoints, request/response shapes (including error envelope from `writeJSONError`). Optionally serve `/openapi.json`.
  - Targets: `api/openapi.yaml` (new), `cmd/modeld/main.go` or `internal/httpapi/server.go` to serve spec.
  - Acceptance: Spec validates and examples in README match.

## 4) Request Validation and Controls

- Configurable request body limit
  - Rationale: 1MiB hardcoded limit in `/infer`.
  - Scope: Add `--max-body-bytes` flag and config key; default 1MiB. Document.
  - Targets: `internal/httpapi/server.go`, `cmd/modeld/main.go`, `internal/config/loader.go`, `README.md`.
  - Acceptance: Changing flag alters allowed body size; tests updated.

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

- Expose backpressure knobs via flags/config
  - Rationale: `ManagerConfig{ MaxQueueDepth, MaxWait }` is used in tests but not exposed to runtime config.
  - Scope: Add config keys and flags; document behavior and defaults.
  - Targets: `cmd/modeld/main.go`, `internal/config/loader.go`, `README.md`.
  - Acceptance: Values applied at startup and affect 429 behavior deterministically.

- Handler/request timeouts
  - Rationale: Prevent indefinite streams.
  - Scope: Add `middleware.Timeout` or explicit context timeouts for `/infer` based on configurable `--infer-timeout`.
  - Targets: `internal/httpapi/server.go`, `cmd/modeld/main.go`.
  - Acceptance: Long-running requests are canceled with appropriate 504/499 mapping.

## 6) Security (Opt-in)

- API key auth
  - Rationale: Minimal protection for non-public deployments.
  - Scope: Support `--auth-mode=none|api_key` and `--api-keys` list or file/env.
  - Targets: `internal/httpapi/server.go` (middleware), `cmd/modeld/main.go`, `README.md`.
  - Acceptance: Requests without valid key receive 401/403; health endpoints may remain open.

- Basic rate limiting
  - Rationale: Shield from accidental overload.
  - Scope: Token-bucket per-IP or per-key; return 429 on exceed.
  - Targets: `internal/httpapi/server.go` (middleware), tests.
  - Acceptance: Deterministic throttling verified by tests.

- TLS/mTLS configuration
  - Rationale: Production-readiness.
  - Scope: Add `--tls-cert`, `--tls-key`, optional client CA for mTLS; document.
  - Targets: `cmd/modeld/main.go`, `README.md`.
  - Acceptance: Server can start with TLS; basic manual verification.

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

- Compression
  - Rationale: Save bandwidth for `/models` and `/status`.
  - Scope: Add `middleware.Compress(5)`.
  - Targets: `internal/httpapi/server.go`.
  - Acceptance: Responses include `Content-Encoding: gzip` when requested.

- CORS
  - Rationale: Browser clients support.
  - Scope: Add configurable CORS (origins, methods, headers) and defaults.
  - Targets: `internal/httpapi/server.go`, `cmd/modeld/main.go`.
  - Acceptance: Preflight succeeds under configured origins.

- Security headers
  - Rationale: Best practices.
  - Scope: Set `X-Content-Type-Options: nosniff` and similar where appropriate.
  - Targets: `internal/httpapi/server.go`.
  - Acceptance: Headers present on responses.

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

- Build info endpoint
  - Rationale: Traceability.
  - Scope: `/build` endpoint returning `version`, `commit`, `built_at`; pass via `-ldflags` in build.
  - Targets: `cmd/modeld/main.go`, `Makefile`, `internal/httpapi/server.go` (route), CI to set ldflags.
  - Acceptance: Endpoint returns info; logs include build fields.

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

- Load and rate tests (optional)
  - Rationale: Backpressure behavior under load.
  - Scope: Add a simple Go or Python script to drive concurrency and assert 429 ratios.
  - Targets: `tests/load/` (new).
  - Acceptance: Script runs locally; not mandatory in CI.

## 12) Security and Privacy

- Prompt handling policy
  - Rationale: Avoid accidental logging of sensitive data.
  - Scope: Configurable prompt logging policy: `redact`, `hash`, or `plain` (default `redact`).
  - Targets: `internal/httpapi/server.go`, `README.md`.
  - Acceptance: Logs reflect policy.

---

## Proposed PR Sequence

1. Observability baseline
   - Structured logging, `/metrics`, max body bytes config, compression.
2. API hardening
   - `/v1` routes, OpenAPI spec, enriched `InferRequest`, and validation.
3. Reliability and config
   - Backpressure flags/config, request timeouts, optional prewarm.
4. Security (opt-in)
   - API key auth and rate limiting.
5. CI/CD and DX
   - Race tests, linting, Dockerfile, build info endpoint, example configs.

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

- Keep backward-compatible aliases during transitions (e.g., old route paths) and deprecate in README.
- When expanding `InferRequest`, ensure tests assert validation and the terminal `{"done": true}` line.
- Avoid logging secrets or full prompts by default. Provide per-request override via headers for debugging sessions.
