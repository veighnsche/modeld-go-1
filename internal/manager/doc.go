// Package manager provides lifecycle, admission, and inference coordination for
// model instances. It is structured into small files by concern:
//
//   - manager.go: core Manager type, constructor, simple getters.
//   - config.go: ManagerConfig and package defaults; NewWithConfig applies defaults.
//   - types.go: internal state types (State, ModelInfo, Instance, Snapshot).
//   - errors.go: error types and helpers (IsTooBusy, IsModelNotFound).
//   - helpers.go: small utilities (model lookup, VRAM estimation).
//   - admission.go: per-instance queueing and generation admission.
//   - ensure.go: EnsureInstance/EnsureModel lifecycle and loading.
//   - evict.go: eviction logic to fit within VRAM budget.
//   - infer.go: inference API entry point and streaming behavior (MVP).
//   - status.go: Status/Snapshot reporting helpers.
//   - ops.go: operational stubs like Switch.
//
// Build tags and runtimes:
//
//   - In-process llama (standard):
//     Uses go-llama.cpp adapter. Enabled with `-tags=llama`.
//     Files: adapter_llamacpp_llama.go, llama_cgo.go (linker rpath hints).
//     A no-CGO stub exists when the tag is not set: adapter_llamacpp.go.
//
//   - External llama_server: DISABLED
//     We standardized on the in-process adapter. The previous llama_server
//     integration is retained only for reference and is excluded from builds.
//     Files excluded via `//go:build ignore`: runtime_llama.go, runtime_llama_stub.go.
//
// External packages should treat this package as the orchestration layer and use
// public methods only (e.g., New/NewWithConfig, Ready, ListModels, Status, Infer).
// Internal types are subject to change.
package manager
