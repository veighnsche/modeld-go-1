#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"
cd "$ROOT_DIR"

step() { echo -e "\n==== $* ====\n"; }

# Install all deps first
step "Install all dependencies (JS, Go, Python)"
bash scripts/install-all.sh

# Run tests without installing again
step "Run full test suite (no install)"
bash scripts/test-all-no-install.sh

step "All test suites completed successfully"
