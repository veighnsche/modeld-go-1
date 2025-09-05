#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"
cd "$ROOT_DIR"

step() { echo -e "\n==== $* ====\n"; }

# 0) Basic tool checks
step "Tool checks"
command -v go >/dev/null || { echo "Go is not installed or not on PATH" >&2; exit 1; }
command -v node >/dev/null || { echo "Node is not installed or not on PATH" >&2; exit 1; }

if ! command -v pnpm >/dev/null 2>&1; then
  echo "pnpm not found; attempting to enable via corepack..."
  if command -v corepack >/dev/null 2>&1; then
    corepack enable || true
    corepack prepare pnpm@9.7.1 --activate || true
  fi
fi

command -v pnpm >/dev/null || { echo "pnpm is required. Install via: npm i -g pnpm (or enable corepack)." >&2; exit 1; }

if ! command -v python3 >/dev/null 2>&1; then
  echo "Python3 is required for black-box tests; please install python3." >&2
  exit 1
fi

# 1) Install JS deps (root + web)
step "Install JS dependencies"
pnpm install --frozen-lockfile
pnpm -C web install --frozen-lockfile

# 2) Go dependencies
step "Go mod download"
go mod download

# 3) Go tests (unit + packages)
step "Run Go tests"
go test ./... -v

# 4) Python E2E black-box tests (uses Makefile helper)
step "Run Python E2E tests"
make e2e-py

# 5) Cypress UI harness - Mock
step "Run Cypress (Mock)"
make e2e-cy-mock

# 6) Cypress UI harness - Live API
step "Run Cypress (Live)"
make e2e-cy-live

step "All test suites completed successfully"
