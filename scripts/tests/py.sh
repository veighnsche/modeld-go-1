#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"
PY_DIR="$ROOT_DIR/tests/e2e_py"
VENV_DIR="$PY_DIR/.venv"

cd "$PY_DIR"
if [[ ! -d "$VENV_DIR" ]]; then
  echo "Python venv not found; running installer..."
  bash "$ROOT_DIR/scripts/install/py.sh"
fi

# shellcheck disable=SC1091
source "$VENV_DIR/bin/activate"

pytest -q "$PY_DIR"
