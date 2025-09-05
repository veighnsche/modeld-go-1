#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: test-cli <command>

Commands:
  install:all        Install JS, Go, Python deps
  install:js         Install JS deps (pnpm root + web)
  install:go         Download Go modules
  install:py         Setup Python venv and pip install

  test:go            Run Go tests
  test:py            Run Python black-box tests
  test:cy:mock       Run Cypress (mock API)
  test:cy:live       Run Cypress (live API)
  test:all           Run all test suites sequentially

  help               Show this help
USAGE
}

cmd=${1:-help}
shift || true

case "$cmd" in
  install:all)
    bash scripts/install-all.sh "$@"
    ;;
  install:js)
    bash scripts/install-js.sh "$@"
    ;;
  install:go)
    bash scripts/install-go.sh "$@"
    ;;
  install:py)
    bash scripts/install-py.sh "$@"
    ;;
  test:go)
    bash scripts/test-go.sh "$@"
    ;;
  test:py)
    make e2e-py "$@"
    ;;
  test:cy:mock)
    make e2e-cy-mock "$@"
    ;;
  test:cy:live)
    make e2e-cy-live "$@"
    ;;
  test:all)
    bash scripts/test-all.sh "$@"
    ;;
  help|--help|-h|*)
    usage
    ;;
 esac
