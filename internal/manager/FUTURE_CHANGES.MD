# Manager: Planned Changes and Iteration Roadmap

This document captures concrete follow-ups and iterations for the `internal/manager/` package. It is intended to be kept up to date as we land changes. Items are grouped by priority and scope.

## Scope

Components covered:

- `adapter_llama_subprocess.go` (spawned llama.cpp server per model path)
- `adapter_llama_server.go` (HTTP client to an existing server, OpenAI-compatible streaming)
- Core manager orchestration: `manager.go`, `config.go`, `instance_ensure.go`, `instance_evict.go`, `queue_admission.go`, `inference.go`, `status_report.go`, `sanity.go`
- Tests across `*_test.go`

## Near-term (P0)

- [ ] Fix data race when reading subprocess process map from `EnsureInstance()`.
  - File: `internal/manager/instance_ensure.go`, function `EnsureInstance()`.
  - Currently reads `sa.procs[mdl.Path]` without `llamaSubprocessAdapter.mu`.
  - Action: add an accessor (e.g., `getProcInfo(modelPath string) (copy, ok)`) on `llamaSubprocessAdapter` that locks and returns a snapshot; call it from `EnsureInstance()`.

- [ ] Use stdlib primitives instead of local re-implementations in `adapter_llama_subprocess.go`.
  - Replace `strconvAtoi()` with `strconv.Atoi`.
  - Replace `ioReadAllLimit()` with `io.ReadAll(io.LimitReader(r, 4096))`.
  - Inline `ioEOF` alias and use `io.EOF` directly.

- [ ] Improve subprocess readiness loop with early-exit on child failure.
  - File: `internal/manager/adapter_llama_subprocess.go`, `ensureProcess()`.
  - Start a goroutine to `Wait()` and surface non-zero exit quickly with context.
  - Return an error including exit status and (optionally) a small stderr tail buffer.

- [ ] Switch JSON body creation to `bytes.NewReader`.
  - Files: `adapter_llama_server.go`, `adapter_llama_subprocess.go`.
  - Replace `strings.NewReader(string(body))` with `bytes.NewReader(body)`.

- [ ] Make panic recovery in `Manager.Infer()` observable.
  - File: `internal/manager/inference.go`, `Infer()`.
  - Log the recovered panic (and stack, if available) or convert to an error that the HTTP layer can surface as 500.

- [ ] Update package docs in `doc.go` to reflect current file names and modes.
  - Fix references: `admission.go` -> `queue_admission.go`, `infer.go` -> `inference.go`, `status.go` -> `status_report.go`, `ops.go` -> `ops_switch.go`.
  - Clarify that server adapter usage is enabled via `ManagerConfig.LlamaServerURL`.

## Mid-term (P1)

- [ ] Graceful subprocess termination in `Stop()`.
  - Attempt `SIGTERM` (or platform equivalent) with timeout before `Kill()`; then `Kill()` as fallback.
  - Consider capturing logs to aid diagnosis on termination.

- [ ] Tighter HTTP client policy and comments.
  - Document that `http.Client{Timeout: 0}` is intentional and all calls must carry context timeouts.
  - Audit request paths to ensure all use context-based timeouts.

- [ ] Error taxonomy improvements.
  - Introduce `budgetExceededError` for capacity constraints instead of overloading `dependencyUnavailableError` in `instance_evict.go`.
  - Map these in the HTTP layer to appropriate status codes (e.g., 429, 507, 503).

- [ ] Background op cancellation policy for `Switch()`.
  - Document intentional detachment from caller context.
  - Provide a top-level shutdown signal (e.g., `Manager.StopAllInstances()` plus a future `Close()` on manager) to end background operations.

- [ ] Remove dead field `Instance.Proc` or wire it properly.
  - Currently unused. Either remove or replace with a typed metadata struct if needed later.

## Longer-term (P2)

- [ ] Observability: structured logs and events.
  - Add logging hooks for state transitions, adapter start/stop, readiness successes/failures, admission backpressure, etc.
  - Optional event bus or subscriber list (`Manager` already hints at subscribers).

- [ ] Admission QoS.
  - Per-model policies, starvation avoidance, and metrics on queue length/latency.
  - Replace `time.After` allocations in hot paths with pooled timers if profiling shows pressure.

- [ ] Adapter extensibility.
  - Validate and extend `InferParams` (e.g., `TopK`, `RepeatPenalty`, seeds) to align with supported adapters.
  - Add a native llama.cpp server streaming path distinct from OpenAI emulation if beneficial.

- [ ] Lifecycle management and model unloading.
  - Add explicit unload hooks and lazy reload policies.
  - Persist LRU metadata and integrate with real memory pressure signals.

## Testing Plan

- [ ] Add tests for subprocess lifecycle in `adapter_llama_subprocess.go`.
  - Simulate a fake `llama-server` by launching a tiny HTTP server binary that prints readiness at `/v1/models`.
  - Cover: port selection (`pickFreePort`, range), readiness success/failure, early exit propagation, and `Stop()` cleanup.

- [ ] Add race tests around `EnsureInstance()` and subprocess map access.
  - Run with `-race` to validate locking discipline.

- [ ] Extend `inference.go` tests to validate panic recovery path logs/behavior.

## Documentation Updates

- [ ] Update `docs/managed-llama-instances-plan.md` to match current capabilities and the spawn-mode behavior (per-model llama.cpp server, ports, PID exposure via `Status()`).
- [ ] Document configuration fields in `ManagerConfig` (server vs spawn mode) and intended precedence.

## Migration Notes

- Stdlib replacements are backward-compatible.
- Error taxonomy changes may require small HTTP layer adjustments.
- Graceful termination is additive; ensure CI accounts for timing changes in tests.

## Open Questions

- Should `Switch()` perform a warmup or full `EnsureInstance()` semantics when multiple models are concurrently requested?
- How should we expose per-session usage accounting (`Usage`) when the adapter streams tokens? Aggregate locally or require adapter to report?
- Do we want a unified streaming protocol abstraction (SSE/NDJSON) between adapters and HTTP layer?

## Reference Patches (to be implemented incrementally)

- Accessor example on subprocess adapter:

```go
// in adapter_llama_subprocess.go
func (a *llamaSubprocessAdapter) getProcInfo(modelPath string) (pid int, baseURL string, ready bool, ok bool) {
    a.mu.Lock()
    defer a.mu.Unlock()
    if a.procs == nil {
        return 0, "", false, false
    }
    if p := a.procs[modelPath]; p != nil {
        return p.pid, p.baseURL, p.ready, true
    }
    return 0, "", false, false
}
```

- Using the accessor in `EnsureInstance()`:

```go
if sa, ok := m.adapter.(*llamaSubprocessAdapter); ok {
    if _, base, _, ok2 := sa.getProcInfo(mdl.Path); ok2 {
        // parse base for port, set inst.Port and inst.PID safely under m.mu
    }
}
```

Keep this file updated as changes land. Each merged item should flip its checkbox to done and include a brief note or PR link.
