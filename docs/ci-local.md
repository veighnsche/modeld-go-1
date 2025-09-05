# Run GitHub Actions CI locally (with `act`)

This repo supports running the Go CI workflow locally using `act` and includes a thin CLI (`bin/testctl`) to help install prerequisites and run local test suites.

Relevant files:
- `.github/workflows/ci-go.yml`
- `.actrc` (maps `ubuntu-latest` to `catthehacker/ubuntu:act-22.04`)
- `Makefile` (local equivalents for lint, swagger, coverage)
- `cmd/testctl/` and `internal/testctl/` (helper CLI)

## Prerequisites
- Docker installed and running
- `act` installed
  - Option A: install via `testctl` (may require sudo for `/usr/local/bin`):
    - `make testctl-build`
    - `bin/testctl install host:act`
  - Option B: install manually from https://github.com/nektos/act

Tip: The included `.actrc` pins the runner image:
```
-P ubuntu-latest=catthehacker/ubuntu:act-22.04
```
This provides good parity with GitHub-hosted Ubuntu runners.

## Quickstart: run the Go CI workflow
- List available jobs:
```
act -l
```
- Dry run (no containers executed):
```
act -n -W .github/workflows/ci-go.yml -j test
```
- Run the `test` job simulating different events:
```
act workflow_dispatch -W .github/workflows/ci-go.yml -j test
act push            -W .github/workflows/ci-go.yml -j test
act pull_request    -W .github/workflows/ci-go.yml -j test
```
- Increase verbosity:
```
act -v -W .github/workflows/ci-go.yml -j test
```
- On ARM hosts (if you hit arch issues):
```
act -P ubuntu-latest=catthehacker/ubuntu:act-22.04 \
    --container-architecture linux/amd64 \
    -W .github/workflows/ci-go.yml -j test
```

## What is skipped under `act`
The workflow uses `if: ${{ env.ACT != 'true' }}` to skip certain steps when running in `act`:
- Verify module files tidy (git diff check)
- Install and run `golangci-lint`
- Upload artifacts
- Upload coverage to Codecov

All other steps will run, including:
- Checkout, setup Go `1.22.x`, cache
- Build (default and `-tags=swagger`)
- Generate Swagger docs via `swag`
- Build `bin/testctl`
- Run tests (coverage subset and race)
- Enforce minimum coverage via `coverage.out`

## Local equivalents for skipped checks
Use the `Makefile` to mimic CI signals locally:
- Lint:
```
make install-golangci-lint
make lint
```
- Swagger docs up-to-date:
```
make swagger-gen
# expect no diff if up-to-date
git diff -- docs
```
- Coverage subset + threshold (80%):
```
make cover-check
```

## Outputs to inspect after running `act`
- `bin/testctl` (built by the workflow)
- `docs/` (Swagger artifacts)
- `coverage.out` (coverage profile)

## Using the `testctl` CLI locally (not GitHub Actions)
`testctl` is a convenience tool and does not execute GitHub Actions by itself.
- Build:
```
make testctl-build
```
- Run local Go tests:
```
bin/testctl test go
```
- Run Python E2E tests:
```
bin/testctl test api:py
```
- Run web UI E2E (auto chooses host models when available):
```
bin/testctl test web auto
# Note: requires host models in ~/models/llm; otherwise errors
```
- Install helpers (Docker / act / language toolchains):
```
bin/testctl install host:all   # host:docker + host:act
bin/testctl install go
bin/testctl install js
bin/testctl install py
```

## Notes and tips
- Path filters in `ci-go.yml` (e.g., only run on `**/*.go`, `go.mod`, `docs/**`, etc.) are not automatically simulated by `act`. If you need to emulate changed files, use a custom event payload (`-e path/to/event.json`).
- The Swagger generation step uses `go run github.com/swaggo/swag/cmd/swag@v1.16.6` and writes to `docs/`. Ensure your working tree is clean before/after runs if you want to detect drift via `git diff`.
- Caching: `actions/cache` is supported in `act` via local volumes, so repeated runs are faster.
