# Manager: Planned Changes and Iteration Roadmap

This document captures concrete follow-ups and iterations for the `internal/manager/` package. It is intended to be kept up to date as we land changes. Items are grouped by priority and scope.

## Scope

Components covered:

- `adapter_llama_subprocess.go` (spawned llama.cpp server per model path)
- `adapter_llama_server.go` (HTTP client to an existing server, OpenAI-compatible streaming)
- Core manager orchestration: `manager.go`, `config.go`, `instance_ensure.go`, `instance_evict.go`, `queue_admission.go`, `inference.go`, `status_report.go`, `sanity.go`
- Tests across `*_test.go`

## Near-term (P0)

- [x] Fix data race when reading subprocess process map from `EnsureInstance()`.
  - File: `internal/manager/instance_ensure.go`, function `EnsureInstance()`.
  - Currently reads `sa.procs[mdl.Path]` without `llamaSubprocessAdapter.mu`.
  - Action: add an accessor (e.g., `getProcInfo(modelPath string) (copy, ok)`) on `llamaSubprocessAdapter` that locks and returns a snapshot; call it from `EnsureInstance()`.
  - Done: implemented `getProcInfo` and used it in `EnsureInstance()` to set `inst.Port` and `inst.PID`. (2025-09-05)

- [x] Use stdlib primitives instead of local re-implementations in `adapter_llama_subprocess.go`.
  - Replace `strconvAtoi()` with `strconv.Atoi`.
  - Replace `ioReadAllLimit()` with `io.ReadAll(io.LimitReader(r, 4096))`.
  - Inline `ioEOF` alias and use `io.EOF` directly.
  - Done: replaced usages and removed helpers. (2025-09-05)

- [x] Improve subprocess readiness loop with early-exit on child failure.
  - File: `internal/manager/adapter_llama_subprocess.go`, `ensureProcess()`.
  - Start a goroutine to `Wait()` and surface non-zero exit quickly with context.
  - Return an error including exit status and (optionally) a small stderr tail buffer.
  - Done: added `cmd.Wait()` watcher and stderr tail capture; cleanup on failure/timeout. (2025-09-05)

- [x] Switch JSON body creation to `bytes.NewReader`.
  - Files: `adapter_llama_server.go`, `adapter_llama_subprocess.go`.
  - Replace `strings.NewReader(string(body))` with `bytes.NewReader(body)`.
  - Done in both adapters. (2025-09-05)

- [x] Make panic recovery in `Manager.Infer()` observable.
  - File: `internal/manager/inference.go`, `Infer()`.
  - Log the recovered panic (and stack, if available) or convert to an error that the HTTP layer can surface as 500.
  - Done: now logs panic with stack and returns an internal error. (2025-09-05)

- [x] Update package docs in `doc.go` to reflect current file names and modes.
  - Fix references: `admission.go` -> `queue_admission.go`, `infer.go` -> `inference.go`, `status.go` -> `status_report.go`, `ops.go` -> `ops_switch.go`.
  - Clarify that server adapter usage is enabled via `ManagerConfig.LlamaServerURL`.
  - Done: doc updated; also documented spawn mode via `SpawnLlama`. (2025-09-05)

## Mid-term (P1)

- [x] Graceful subprocess termination in `Stop()`.
  - Attempt `SIGTERM` (or platform equivalent) with timeout before `Kill()`; then `Kill()` as fallback.
  - Consider capturing logs to aid diagnosis on termination.

- [ ] Tighter HTTP client policy and comments.
  - Document that `http.Client{Timeout: 0}` is intentional and all calls must carry context timeouts.
  - Audit request paths to ensure all use context-based timeouts.
  - Done: added comments and verified adapters use context-bound requests with deadlines. (2025-09-05)
  - [x] Completed

- [x] Error taxonomy improvements.
  - Introduce `budgetExceededError` for capacity constraints instead of overloading `dependencyUnavailableError` in `instance_evict.go`.
  - Map these in the HTTP layer to appropriate status codes (e.g., 429, 507, 503).
  - Done: added `ErrBudgetExceeded` and mapped to HTTP 507 in `internal/httpapi/server.go`. (2025-09-05)

- [x] Background op cancellation policy for `Switch()`.
  - Document intentional detachment from caller context.
  - Provide a top-level shutdown signal (e.g., `Manager.StopAllInstances()` plus a future `Close()` on manager) to end background operations.

- [x] Remove dead field `Instance.Proc` or wire it properly.
  - Currently unused. Either remove or replace with a typed metadata struct if needed later.

## Longer-term (P2)

- [x] Observability: structured logs and events.
  - Add logging hooks for state transitions, adapter start/stop, readiness successes/failures, admission backpressure, etc.
  - Event bus: introduce a simple publisher interface and emit events for ensure/ready/evict/spawn start/stop to enable richer observability without parsing logs.
  - Progress: added adapter-level logs (start/ready/timeout/exit) in `adapter_llama_subprocess.go` and stream debug logs in `adapter_llama_server.go`; exposed `State` and `WarmupsInProgress` in `/status`. (2025-09-06)
  - Done: implemented `EventPublisher`, integrated across ensure/spawn/unload, and added CLI flags to stream events to stdout/file. (2025-09-06)

- [ ] Admission QoS.
  - Per-model policies, starvation avoidance, and metrics on queue length/latency.
  - Replace `time.After` allocations in hot paths with pooled timers if profiling shows pressure.
  - Done (timers): replaced `time.After` with pooled timers in `queue_admission.go`. (2025-09-06)

- [x] Adapter extensibility.
  - Validate and extend `InferParams` (e.g., `TopK`, `RepeatPenalty`, seeds) to align with supported adapters.
  - Add a native llama.cpp server streaming path distinct from OpenAI emulation if beneficial.
  - Done: extended `pkg/types.InferRequest` with `top_k` and `repeat_penalty`, mapped in `inference.go`, adapters already propagate. (2025-09-06)

- [x] Lifecycle management and model unloading.
  - Add explicit `Unload(modelID)` hook on `Manager`.
  - Wire eviction with a configurable "graceful drain" window before forceful stop (allow in-flight to finish, reject new enqueues).
  - Create a basic LRU metadata stub (last-used, size hints) for future persistence and integration with real memory pressure signals.
  - Plan: expose unload and drain progress via events and `/status`.
  - Done: implemented `Unload()`, added `StateDraining`, reject new enqueues during drain; exposed `DrainingCount` in `/status` and emitted unload events. (2025-09-06)

## Testing Plan

- [x] Add tests for subprocess lifecycle in `adapter_llama_subprocess.go`.
  - Simulate a fake `llama-server` by launching a tiny HTTP server binary that prints readiness at `/v1/models`.
  - Cover: port selection (`pickFreePort`, range), readiness success/failure, early exit propagation, and `Stop()` cleanup.
  - Done: added `internal/manager/testdata/fake_llama_server.go` and `adapter_llama_subprocess_test.go`. (2025-09-05)

- [x] Add race tests around `EnsureInstance()` and subprocess map access.
  - Run with `-race` to validate locking discipline.
  - Done: added `TestSubprocessAccessorRace`. (2025-09-05)

- [x] Extend `inference.go` tests to validate panic recovery path logs/behavior.
  - Done: added `inference_panic_test.go`. (2025-09-05)

## Documentation Updates

- [x] Update `docs/managed-llama-instances-plan.md` to match current capabilities and the spawn-mode behavior (per-model llama.cpp server, ports, PID exposure via `Status()`).
- [x] Document configuration fields in `ManagerConfig` (server vs spawn mode) and intended precedence.
  - Done: updated plan doc and added a Configuration section in `docs/build-and-run.md`. (2025-09-05)

## Migration Notes

- Stdlib replacements are backward-compatible.
- Error taxonomy changes may require small HTTP layer adjustments.
- Graceful termination is additive; ensure CI accounts for timing changes in tests.

## Open Questions

- [x] Should `Switch()` perform a warmup or full `EnsureInstance()` semantics when multiple models are concurrently requested?
  - Decision: perform warmup only.
- [x] How should we expose per-session usage accounting (`Usage`) when the adapter streams tokens? Aggregate locally or require adapter to report?
  - Decision: require adapter to report `Usage`; manager will surface/forward adapter-reported usage without local aggregation.
- [x] Do we want a unified streaming protocol abstraction (SSE/NDJSON) between adapters and HTTP layer?
  - Decision: unified NDJSON between adapters and the HTTP layer; adapters should normalize streaming outputs to NDJSON lines.

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
