# Testing in `internal/manager/`

This document explains how tests in `internal/manager/` are structured, how to run them, and the conventions to follow when adding new tests.

## Test tiers

There are two tiers of tests:

- Unit tests (default): Fast, hermetic tests with no external dependencies. These run with the default `go test` invocation and in CI by default.
- Integration tests: Heavier tests that exercise subprocess spawning, networking, or timing-sensitive behavior. These are excluded by default via Go build tags and must be explicitly enabled.

## Running tests

- Run the whole repository (legacy):
  ```bash
  make test
  ```

- Run only unit tests for `internal/manager/` (hermetic):
  ```bash
  make test-unit
  # or
  go test ./internal/manager -count=1 -v
  ```

- Run integration tests for `internal/manager/`:
  ```bash
  make test-integration
  # or
  go test -tags=integration ./internal/manager -count=1 -v
  ```

## Build tags

Integration tests are marked with the build tag:

```go
//go:build integration
// +build integration
```

This keeps them out of the default unit suite. Files currently tagged include:

- `adapter_llama_server_test.go`
- `adapter_llama_server_unknown_test.go`
- `adapter_llama_subprocess_test.go`
- `adapter_llama_subprocess_utils_test.go`
- `adapter_llama_subprocess_generate_test.go`
- `adapter_llama_subprocess_stop_test.go`
- `adapter_llama_subprocess_earlyexit_test.go`
- `close_test.go`

## Shared test helpers

Common helpers live in `internal/manager/testutil_test.go`:

- `createModelFile(t, dir, name, sizeMB string)`: Create a file of a given size.
- `fakeAdapter`, `fakeSession`: In-memory inference adapter and session for streaming tests.
- `errWriter`: Writer that errors after the first write to test error paths.
- `testCtx(t)`: Context helper with a short timeout and automatic cleanup.

When you need a helper across multiple tests, add it to `testutil_test.go` and remove local duplicates.

## File organization conventions

- Group tests by feature area and keep file names descriptive:
  - `inference_*_test.go`: streaming, tokenization, panic recovery, error paths.
  - `manager_*_test.go`: lifecycle, snapshot/status, LRU eviction policy.
  - `queue_*_test.go`: admission and timeouts.
  - `config_*_test.go`: default and override behaviors.
  - `preflight_*_test.go`: path checks.
  - `ports_test.go`: simple port utility tests (unit) — any non-deterministic variants go under integration.
- Prefer table-driven tests where scenarios are similar.
- Avoid timers and sleeps in unit tests. If unavoidable, move to integration or gate with build tags.

## Flake control

- Do not bind to fixed ports in unit tests. Use in-memory fakes or stub HTTP servers that do not require fixed port ranges.
- Any tests that spawn subprocesses or rely on timing/network should be marked as integration.
- Keep sleeps minimal and protected by timeouts from `testCtx(t)`.

## Adding/Updating tests

1. Decide unit vs integration based on dependencies and determinism.
2. Use shared helpers from `testutil_test.go`.
3. Keep assertions focused on behavior and avoid duplicating coverage across files.
4. If a new helper is needed broadly, add it to `testutil_test.go`.

## CI considerations

- CI should run unit tests on every change, and integration tests as a separate job (nightly or gated).
- To run integration tests in CI, pass `-tags=integration` to `go test`.
