#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)"
cd "$ROOT_DIR"

CLI="scripts/cli/test.sh"

pass_count=0
fail_count=0

tap_ok() {
  echo "ok - $1"
  pass_count=$((pass_count+1))
}

tap_fail() {
  echo "not ok - $1"
  fail_count=$((fail_count+1))
}

# 1) Help (non-TTY, no args)
if output=$(echo | bash "$CLI" 2>&1); then
  if grep -q "Usage: test-cli" <<<"$output"; then
    tap_ok "help shown when no args in non-TTY"
  else
    echo "$output"
    tap_fail "help missing in non-TTY no-arg invocation"
  fi
else
  echo "$output"
  tap_fail "help invocation exited non-zero"
fi

# 2) Version flag
if output=$(bash "$CLI" --version 2>&1); then
  if grep -q "test-cli version" <<<"$output"; then
    tap_ok "version flag works"
  else
    echo "$output"
    tap_fail "version output missing"
  fi
else
  echo "$output"
  tap_fail "version invocation exited non-zero"
fi

# 3) Subcommand routing (dry-run)
check_dryrun() {
  local subcmd="$1"; shift
  local expect="$1"; shift
  if output=$(TEST_CLI_DRYRUN=1 bash "$CLI" "$subcmd" 2>&1); then
    if grep -q "DRYRUN: $expect" <<<"$output"; then
      tap_ok "dry-run routes $subcmd -> $expect"
    else
      echo "$output"
      tap_fail "dry-run output mismatch for $subcmd (expected: $expect)"
    fi
  else
    echo "$output"
    tap_fail "dry-run invocation exited non-zero for $subcmd"
  fi
}

check_dryrun "install:all" "bash scripts/install/all.sh"
check_dryrun "install:js" "bash scripts/install/js.sh"
check_dryrun "install:go" "bash scripts/install/go.sh"
check_dryrun "install:py" "bash scripts/install/py.sh"
check_dryrun "test:go" "bash scripts/tests/go.sh"
check_dryrun "test:py" "make e2e-py"
check_dryrun "test:cy:mock" "make e2e-cy-mock"
check_dryrun "test:cy:live" "make e2e-cy-live"
check_dryrun "test:all" "bash scripts/tests/all.sh"

# 4) Invalid command falls back to help
if output=$(bash "$CLI" does-not-exist 2>&1); then
  if grep -q "Usage: test-cli" <<<"$output"; then
    tap_ok "invalid command shows help"
  else
    echo "$output"
    tap_fail "invalid command did not show help"
  fi
else
  echo "$output"
  tap_fail "invalid command exited non-zero"
fi

# Summary
plan=$((pass_count+fail_count))
echo "1..$plan"  # TAP plan at end for loose parsers
if [[ $fail_count -gt 0 ]]; then
  echo "$fail_count test(s) failed" >&2
  exit 1
else
  echo "All $pass_count test(s) passed"
fi
