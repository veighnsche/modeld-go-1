# Migration Plan: Replace go-llama.cpp with llama.cpp Server API

This document outlines the end-to-end plan to replace the in-process `go-llama.cpp` integration with an HTTP client that talks to the llama.cpp API server (`llama-server`). It is designed for incremental, low-risk rollout with clear configuration, testing, and deprecation steps.


## Goals

- Replace CGO + `go-llama.cpp` adapter with an HTTP-based adapter to `llama-server`.
- Maintain current manager and HTTP API surfaces (`/infer`, streaming NDJSON tokens, backpressure semantics).
- Provide robust error handling, cancellation, and streaming behavior.
- Keep clean fallbacks and clear deprecation path for the CGO integration.


## Non-Goals

- Changing the public HTTP API contract of `modeld`.
- Multi-backend support beyond llama.cpp (can be future work via the same adapter interface).


## Current Architecture Summary

- Manager uses `InferenceAdapter` (`internal/manager/adapter_iface.go`).
- `adapter_llama.go` (build tag `llama`) implements the adapter via `go-llama.cpp` and CGO.
- `adapter_llama_stub.go` (build tag `!llama`) returns a stub that errors at runtime.
- `llama_cgo.go` contains link directives and relies on `libllama.so` in `./bin` at runtime.
- `Manager.NewWithConfig` unconditionally initializes the in-process llama adapter today.


## Target Architecture (HTTP, no CGO)

- New adapter: `internal/manager/adapter_llama_server.go`.
  - Implements `InferenceAdapter` using an HTTP client.
  - Talks to a running `llama-server` process.
  - Uses streaming responses to feed NDJSON token lines to callers.
- Configuration supplies the server base URL and optional API key and timeouts.
- `Manager.NewWithConfig` picks the server adapter when configured; otherwise leaves adapter nil so `/infer` fails fast with dependency unavailable (as it already does when adapter is nil).
- Remove CGO dependency and `llama` build tag from default builds.


## llama.cpp Server API Surfaces

llama.cpp server exposes two families of endpoints:

- OpenAI-compatible endpoints (preferred):
  - `POST /v1/chat/completions` for chat-style requests (supports streaming via `stream: true`).
  - `POST /v1/completions` for prompt-style requests.
  - `GET /v1/models` to list models; some builds support dynamic load/unload.
- Native endpoints (legacy/alt):
  - `POST /completion` and `POST /completion-stream`.

We will target OpenAI-compatible endpoints by default for best forward compatibility, with a fallback to native endpoints if needed.


## Request/Response Mapping

Given our `types.InferRequest` and `manager.InferParams`, we will map to OpenAI-compatible requests as follows:

- Model selection:
  - Use `req.Model` to set the OpenAI `model` field. If the server is launched with a single model, the field may be optional, but we will always attempt to send it for clarity.
- Prompt and stop:
  - For completions: `prompt: req.Prompt`, `stop: req.Stop`.
  - For chat-completions: we can wrap as a single-user message; initial implementation will use completions for parity with the current prompt-based API.
- Sampling and limits:
  - `temperature: req.Temperature`.
  - `top_p: req.TopP`.
  - `max_tokens: req.MaxTokens` (omit/zero to let server default).
  - `seed: req.Seed` if supported by server build.
  - `top_k` and `repeat_penalty` included when present.
- Streaming:
  - Set `stream: true` and parse the stream.

Server stream protocol options:

- OpenAI SSE: `data: {"id":"...","object":"...","choices":[{"delta":{"content":"..."}}]}`. Ended by `data: [DONE]`.
- Native chunked JSON: lines containing partial token strings.

We will implement both parsers with detection, preferring OpenAI SSE when returned.


## Manager and Adapter Changes

- Add a new file `internal/manager/adapter_llama_server.go`:
  - Implements `InferenceAdapter` using a reusable `http.Client` with tuned timeouts.
  - `Start(modelPath string, params InferParams) (InferSession, error)` ignores `modelPath` for server mode or validates against configured/default model; instead, passes `req.Model` to the server.
  - `Generate(ctx, prompt, onToken)`: performs `POST /v1/completions` (or `/v1/chat/completions`), sets `stream:true`, parses streamed chunks, and calls `onToken` with each token fragment.
  - Maps final usage fields when available; otherwise, returns zeros (parity with today’s limited accounting).
  - Properly returns on context cancellation and translates timeouts to `context.DeadlineExceeded`.
  - Error mapping: 4xx -> `ErrDependencyUnavailable` only for configuration/compat cases; otherwise propagate as `adapter generate` error.

- Update `internal/manager/config.go::NewWithConfig`:
  - Do not initialize in-process adapter by default.
  - If `LlamaServerURL != ""`, initialize `adapter_llama_server` with config.
  - Keep `m.adapter == nil` case to preserve fast-fail without mocks.

- Keep `adapter_llama_stub.go` as the default when no server URL provided to maintain deterministic behavior in environments without llama.


## Configuration and CLI Changes

Add new config fields and flags. Keep legacy flags temporarily but mark as deprecated.

- New ManagerConfig fields:
  - `LlamaServerURL string` (e.g., `http://127.0.0.1:8081`)
  - `LlamaAPIKey string` (optional; for `Authorization: Bearer`)
  - `LlamaRequestTimeout time.Duration` (per-request)
  - `LlamaConnectTimeout time.Duration` (dial timeout)
  - `LlamaUseOpenAI bool` (prefer OpenAI endpoints, default true)

- CLI flags in `cmd/modeld/main.go`:
  - `--llama-url` (string)
  - `--llama-api-key` (string)
  - `--llama-timeout` (duration, request timeout; default 30s)
  - `--llama-connect-timeout` (duration, dial timeout; default 5s)
  - `--llama-use-openai` (bool; default true)

- Config file support in `internal/config/loader.go` (optional but recommended):
  - Add corresponding fields to the parsed config (yaml/json/toml) and wire into `main.go` with the same CLI-precedence logic already used.

- Deprecations:
  - `--llama-bin`, `--llama-ctx`, `--llama-threads` become deprecated. Keep parsing but do not use when `--llama-url` is set.
  - Add startup warnings if deprecated flags are set and ignored.


## Preflight Checks

Augment `Manager.Preflight()` to include server mode checks when `LlamaServerURL` is set:

- Check reachability: `GET /v1/models` or `GET /health` within `connect-timeout`.
- Validate default model (if configured) exists in `/v1/models` list.
- Validate feature support: try a dry-run `POST /v1/completions` with `max_tokens: 1, stream: false` when allowed, or skip if server disallows.
- Emit clear messages and fail the preflight block if server not reachable or model not available.


## Streaming Implementation Details

- Use a single `http.Client` per adapter with custom `Transport`:
  - Set `DialContext` with connect timeout.
  - Reasonable idle connections (keep-alive) and TLS config.
- For OpenAI SSE:
  - Parse line by line; for `data: [DONE]` stop.
  - Accumulate `choices[0].delta.content` fragments; call `onToken` per fragment.
- For native chunked JSON (fallback):
  - Parse NDJSON or raw token strings per chunk; call `onToken` per token.
- Cancellation:
  - On `ctx.Done()`, close the response body and return.


## Error Handling and Mapping

- Connectivity errors: map to `ErrDependencyUnavailable("llama adapter not initialized or server unreachable")` on startup; during requests, propagate as `adapter generate: <err>`.
- HTTP status >= 400: parse JSON body `{error: {message, type}}` when present; include in wrapped error.
- Timeouts: translate to context errors (`context.DeadlineExceeded`) consistently.


## Testing Strategy

- Unit tests for the adapter:
  - Mock HTTP server emitting OpenAI SSE and native formats, including `[DONE]`, partials, and malformed events.
  - Cancellation tests: ensure we stop on context cancel.
  - Error mapping tests for 4xx/5xx.

- E2E tests:
  - Add a Docker-based `llama.cpp` server service for CI (optional: use a tiny model or a fake streaming server)
  - Update `e2e/specs` to exercise `/infer` happy path and error cases with streaming.
  - Provide a lightweight mock server for CI if real model loading is impractical.

- Local dev:
  - Document how to run `llama-server`: `llama-server -m /path/to/model.gguf -c 4096 -ngl 0 -t 4 --host 127.0.0.1 --port 8081`.


## Build, Makefile, and Repo Cleanup

- Remove CGO build tag dependence from default build; ultimately delete:
  - `internal/manager/adapter_llama.go`
  - `internal/manager/llama_cgo.go`
  - `bin/libllama.so` artifact and related linker expectations
- Update `Makefile` targets:
  - Remove `-tags=llama` guidance.
  - Add `LLAMA_URL` env and/or flags to run against server in `dev-run.sh` and docs.

- Update `docs` and `README.md`:
  - Add a new section for running with `llama-server`.
  - Mark CGO/go-llama.cpp path as deprecated and scheduled for removal in a release.

- Add `docs/env-examples/llama-server.env.example` with `LLAMA_URL` and optional auth.


## Rollout Plan

- Phase 1 (Introduce server adapter behind config):
  - Ship new adapter and config flags.
  - Keep CGO adapter and build tag working for a compatibility window.

- Phase 2 (Default to server adapter):
  - Stop instantiating CGO adapter by default; require `--llama-url`.
  - Print deprecation warnings if CGO flags are used.

- Phase 3 (Removal):
  - Remove CGO code and `llama` tag.
  - Remove related flags and docs.


## Backward Compatibility and Fallbacks

- When `LlamaServerURL` is not set, `m.adapter` remains nil and `/infer` will return `dependency unavailable`, same as today when not built with `llama`.
- Maintain NDJSON token streaming from the HTTP layer so clients are unaffected.


## Work Items Checklist

- Adapter design and implementation
  - [ ] `internal/manager/adapter_llama_server.go`
  - [ ] HTTP client with timeouts and SSE/native parsing
  - [ ] Usage and finish_reason mapping

- Config and wiring
  - [ ] Add fields to `ManagerConfig` and `Manager`
  - [ ] Update `NewWithConfig` factory logic
  - [ ] CLI flags in `cmd/modeld/main.go`
  - [ ] Optional config file integration

- Preflight and sanity
  - [ ] Reachability and models check
  - [ ] Clear error messages in preflight summary

- Tests
  - [ ] Unit tests for adapter (SSE, native, errors, cancel)
  - [ ] E2E setup targeting `llama-server`

- Docs and cleanup
  - [ ] Update README and docs
  - [ ] Add env example
  - [ ] Deprecate CGO and plan removal


## Open Questions / Assumptions

- Some llama.cpp builds may not expose OpenAI-compatible endpoints or may differ in JSON schema. We will implement endpoint negotiation with a simple probe: try `/v1/models`; if 404, probe native endpoints.
- Dynamic model hot-loading may vary by version; initially assume server is started with the desired model and we pass the `model` field for future compatibility.
- Token-level usage accounting may not be available; we will return zeros unless provided by the server.


## Appendix: Example Requests

- OpenAI-style completion request (streaming):

```json
POST /v1/completions
{
  "model": "<model-id-or-path>",
  "prompt": "<prompt>",
  "max_tokens": 128,
  "temperature": 0.7,
  "top_p": 0.95,
  "stop": ["\n\n"],
  "stream": true
}
```

- Native llama.cpp (fallback) is similar but may use `/completion` and different field names; we will adapt in the adapter as needed.
