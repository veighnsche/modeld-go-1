# Managed llama.cpp Instances: Plan and Migration

## Context and Problem Statement

- Our current adapter (`internal/manager/adapter_llama_server.go`) assumes an already-running external llama.cpp server and talks to it via HTTP (OpenAI-style endpoints).
- Product requirements and previous feature work (VRAM budgeting, per-model instances, readiness/eviction, queueing/backpressure) assume the Manager owns the lifecycle of llama.cpp processes so multiple models can run concurrently on a single host.
- The mismatch causes friction: tests need `LLAMA_URL`, and we can’t demonstrate the "manager manages instances" story end-to-end.

## Goals

- Provide a subprocess-backed adapter that launches and manages a `llama-server` process per model instance.
- Integrate with the existing Manager lifecycle and backpressure mechanisms without re-architecting the HTTP API layer.
- Enable multi-model concurrency, readiness reporting, and a path to VRAM budgeting.
- Keep external-server mode as an option (useful for remote clusters or hosted llama services).

## Non-Goals (for Phase 1)

- Full VRAM-aware scheduling with precise CUDA accounting.
- Auto-tuning of threads, context size, batching, or quantization.
- Hot-swapping model weights within a single `llama-server` process.

---

## Current State Summary

- `internal/manager/`:
  - `instance_ensure.go`, `instance_evict.go`: implement ensure/evict semantics for model instances and are used by inference.
  - `queue_admission.go`: implements per-instance FIFO queueing/backpressure with `MaxQueueDepth` and `MaxWait`.
  - `inference.go`: calls `EnsureInstance`, starts an adapter `InferSession`, streams NDJSON tokens, and writes a final line.
  - `adapter_llama_server.go`: HTTP client to external llama server.
  - `config.go`: builds a `Manager` and picks the external adapter if `LlamaServerURL` is set.
- `internal/httpapi/`: stable HTTP surface including `/models`, `/status`, `/infer`, health and metrics.
- `cmd/modeld/`: CLI + config loader set for external-server adapter.

Conclusion: the Manager lifecycle, queueing, and HTTP plumbing are compatible with a subprocess adapter. This has now been implemented: the manager can spawn a `llama-server` per model and manage its lifecycle.

---

## Design Summary

### Adapter: `adapter_llama_subprocess`

- Implementation of `InferenceAdapter` that:
  - On `Start(modelPath, params)`, ensures a long-lived `llama-server` process exists for the target model. If missing, spawns one.
  - Picks a free TCP port per process and starts `llama-server` with flags derived from config (e.g., `-m <path> -c <ctx> -ngl <N> -t <threads> --host 127.0.0.1 --port <X>`).
  - Waits for readiness by polling `/v1/models` (with deadlines and early-exit on child failure).
  - Returns a session whose `Generate` forwards to the local `llama-server` using OpenAI-compatible streaming logic.
  - `Close()` on the session does not kill the process; instance eviction kills the process.

### Manager Integration

- Instance state (`Instance`) holds:
  - PID of the managed subprocess and its assigned port (exposed via `/status`).
  - Ready/Loading/Error state transitions reusing existing readiness checks.
- `EnsureInstance(ctx, modelID)`:
  - If not present, admits into creation gate, spawns subprocess for the model’s `.gguf`, waits for readiness (or returns error).
  - If present, verifies health; optionally restart policy (phase 2).
- `EvictInstance(modelID)`:
  - Gracefully terminates the subprocess.
  - Cleans up port allocation and state.

### Why Queues Are Still Needed

- Queues protect each instance from overload and ensure fair FIFO admission when multiple requests target the same model. They also enable controlled backpressure via `MaxQueueDepth` and `MaxWait`.
- With subprocess management, each instance is still single-producer (one request at a time) unless `llama-server` supports sufficient concurrency. The queue is the safety valve.

### Config Changes

`ManagerConfig` selects adapter mode and passes spawn options:

- `LlamaServerURL string` to use an external llama.cpp server (OpenAI-compatible).
- `SpawnLlama bool` and `LlamaBin string` to enable spawn mode.
- `LlamaHost string` (default `127.0.0.1`).
- `LlamaPortStart, LlamaPortEnd int` (optional port range; 0=auto).
- `LlamaThreads int`, `LlamaCtxSize int`, `LlamaNGL int`.
- `LlamaExtraArgs []string`.

Adapter selection (`NewWithConfig`): spawn mode takes precedence when enabled; otherwise server mode is used if `LlamaServerURL` is set.

### Instance Lifecycle

- Creation:
  1) EnsureInstance enter critical section per model.
  2) Allocate port and start `llama-server` subprocess.
  3) Poll readiness (deadline configurable).
  4) Mark instance ready and publish in `/status`.
- Inference:
  1) Admission: `beginGeneration` acquires the in-flight token; others queue.
  2) Session: adapter forwards to `http://127.0.0.1:<port>/v1/completions` and streams tokens.
- Eviction:
  1) Reject new requests, wait for in-flight to finish or timeout.
  2) Send SIGTERM, wait grace period, then SIGKILL if needed.
  3) Free port and resources; update status.

---

## Compatibility and Migration

- `instance_ensure.go` / `instance_evict.go` remain valid; they will now call into subprocess adapter functions to create/kill processes instead of assuming external servers.
- `queue_admission.go` stays as-is and is critical to protect instances.
- No changes needed in `httpapi` or NDJSON wire format.
- External-server mode continues to work unchanged; spawn mode is opt-in.

---

## Testing Plan

- Unit tests for subprocess utilities:
  - Port allocation.
  - Command construction from config.
  - Readiness polling with timeouts.
- Integration tests (skipping when `llama-server` or `.gguf` unavailable) are planned.
- Existing e2e (CORS, 429, status, metrics) remain in place.

---

## Phased Implementation

- Phase 1 (MVP):
  - Subprocess adapter for one process per model.
  - Lazy start on first request; simple readiness check; no restart policy.
  - Basic CLI and config flags.
  - E2E smoke test for haiku in spawn mode.
- Phase 2 (Operational hardening):
  - VRAM budgeting signals and refusal logic (soft limits).
  - Port pool management and metrics.
  - Process restart policy; crash detection.
  - More robust health checks and timeout tuning.

---

## Risks and Mitigations

- Process leaks: ensure robust cleanup paths (defer, signal handling, shutdown hooks).
- Port conflicts: allocate ports atomically and hold them while process runs.
- Readiness flakiness: configurable backoff and timeouts; surface clear errors.
- Cross-platform behavior: document Linux/macOS as supported; Windows best-effort.

---

## Deliverables Checklist

- [ ] `internal/manager/adapter_llama_subprocess.go` with Start/Generate/Close
- [ ] ManagerConfig + CLI flags for spawn mode
- [ ] Adapter selection logic in `NewWithConfig`
- [ ] Tests: unit + e2e smoke
- [ ] Documentation: this plan, README updates, example configs
