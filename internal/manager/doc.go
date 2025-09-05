// Package manager provides lifecycle, admission, and inference coordination for
// model instances. It is structured into small files by concern:
//
//   - manager.go: core Manager type, constructor, simple getters.
//   - config.go: ManagerConfig and package defaults; NewWithConfig applies defaults.
//   - types.go: internal state types (State, ModelInfo, Instance, Snapshot).
//   - errors.go: error types and helpers (IsTooBusy, IsModelNotFound).
//   - helpers.go: small utilities (model lookup, VRAM estimation).
//   - queue_admission.go: per-instance queueing and generation admission.
//   - instance_ensure.go: EnsureInstance/EnsureModel lifecycle and loading.
//   - instance_evict.go: eviction logic to fit within VRAM budget.
//   - inference.go: inference API entry point and streaming behavior (MVP).
//   - status_report.go: Status/Snapshot reporting helpers.
//   - ops_switch.go: operational stubs like Switch.
//
// Build tags and runtimes:
//
//   - External llama.cpp server adapter:
//     When ManagerConfig.LlamaServerURL is set, the manager will use an HTTP client
//     adapter to talk to an already-running llama.cpp server (OpenAI-compatible endpoints).
//
//   - Subprocess-managed llama.cpp (spawn mode):
//     When ManagerConfig.SpawnLlama is true and LlamaBin is provided, the manager will
//     spawn a local `llama-server` per model instance and manage its lifecycle.
//
// External packages should treat this package as the orchestration layer and use
// public methods only (e.g., New/NewWithConfig, Ready, ListModels, Status, Infer).
// Internal types are subject to change.
package manager
