#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"
cd "$ROOT_DIR"

step() { echo -e "\n==== $* ====\n"; }

# 1) Go tests (unit + packages)
step "Run Go tests"
go test ./... -v

# 2) Python E2E black-box tests
step "Run Python E2E tests"
make e2e-py

# 3) Cypress UI harness - Mock
step "Run Cypress (Mock)"
make e2e-cy-mock

# 4) Cypress UI harness - Live API
step "Run Cypress (Live)"
make e2e-cy-live

step "All test suites completed successfully"
