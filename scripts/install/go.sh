#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)"
cd "$ROOT_DIR"

go mod download

echo "Go modules downloaded."
