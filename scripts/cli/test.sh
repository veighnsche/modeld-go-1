#!/usr/bin/env bash
set -euo pipefail

VERSION="0.1.0"

# If TEST_CLI_DRYRUN=1, we will print commands instead of executing them.
run_cmd() {
  if [[ "${TEST_CLI_DRYRUN:-0}" = "1" ]]; then
    echo "DRYRUN:" "$@"
  else
    "$@"
  fi
}

usage() {
  cat <<'USAGE'
Usage: test-cli <command>

Description:
  A lightweight Bash CLI to install dependencies and run tests across the repo.
  Designed to work on fresh Arch installs without Node; only Bash + system tools required.

Commands:
  install:all        Install JS, Go, and Python dependencies
  install:js         Install JS deps (pnpm at root + web/)
  install:go         Download Go modules
  install:py         Create venv under tests/e2e_py/.venv and pip install

  test:go            Run Go tests (go test ./... -v)
  test:py            Run Python black-box tests (pytest tests/e2e_py)
  test:cy:mock       Run Cypress against the mock web harness
  test:cy:live       Run Cypress against a live API started by Makefile
  test:all           Run all test suites sequentially

  help               Show this help
  --version          Print CLI version

Interactive mode:
  Run without arguments to open an interactive menu (when attached to a TTY).

Prerequisites:
  - Go toolchain (go)
  - Python 3 (python3)
  - Node + pnpm for JS/Cypress (CLI will try to enable pnpm via corepack if present)

Environment variables (used by downstream scripts):
  - WEB_PORT            Port for Vite preview (default: 5173)
  - CYPRESS_BASE_URL    Base URL for Cypress runs
  - CYPRESS_USE_MOCKS   "1" for mock mode, "0" for live API mode
  - CYPRESS_API_*       API health/ready/status URLs in live mode

Examples:
  bash scripts/cli/test.sh install:all
  bash scripts/cli/test.sh test:go
  bash scripts/cli/test.sh test:cy:mock
  bash scripts/cli/test.sh test:all

Exit codes:
  0 on success; non-zero if any subcommand fails.
USAGE
}

# Interactive menu if no arguments and running in a TTY
if [[ $# -eq 0 && -t 0 ]]; then
  echo "Select an action:"
  PS3="> "
  options=(
    "install:all"
    "install:js"
    "install:go"
    "install:py"
    "test:go"
    "test:py"
    "test:cy:mock"
    "test:cy:live"
    "test:all"
    "help"
    "quit"
  )
  select opt in "${options[@]}"; do
    case "$opt" in
      quit)
        exit 0
        ;;
      "" )
        echo "Invalid selection" >&2
        ;;
      *)
        exec bash "$0" "$opt"
        ;;
    esac
  done
fi

cmd=${1:-help}
shift || true

case "$cmd" in
  install:all)
    run_cmd bash scripts/install/all.sh "$@"
    ;;
  install:js)
    run_cmd bash scripts/install/js.sh "$@"
    ;;
  install:go)
    run_cmd bash scripts/install/go.sh "$@"
    ;;
  install:py)
    run_cmd bash scripts/install/py.sh "$@"
    ;;
  test:go)
    run_cmd bash scripts/tests/go.sh "$@"
    ;;
  test:py)
    run_cmd make e2e-py "$@"
    ;;
  test:cy:mock)
    run_cmd make e2e-cy-mock "$@"
    ;;
  test:cy:live)
    run_cmd make e2e-cy-live "$@"
    ;;
  test:all)
    run_cmd bash scripts/tests/all.sh "$@"
    ;;
  --version)
    echo "test-cli version $VERSION"
    ;;
  help|--help|-h|*)
    usage
    ;;
 esac
