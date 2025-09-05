#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"
PY_DIR="$ROOT_DIR/tests/e2e_py"
VENV_DIR="$PY_DIR/.venv"

command -v python3 >/dev/null 2>&1 || { echo "python3 is required" >&2; exit 1; }

cd "$PY_DIR"
if [[ ! -d "$VENV_DIR" ]]; then
  echo "Creating virtualenv in $VENV_DIR"
  python3 -m venv "$VENV_DIR"
fi

# shellcheck disable=SC1091
source "$VENV_DIR/bin/activate"

python -m pip install --upgrade pip
pip install -r requirements.txt

echo "Python dependencies installed (venv: $VENV_DIR)."
