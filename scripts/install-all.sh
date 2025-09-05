#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"
cd "$ROOT_DIR"

bash scripts/install-js.sh
bash scripts/install-go.sh
bash scripts/install-py.sh

echo "All dependencies installed (JS, Go, Python)."
