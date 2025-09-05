#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"
cd "$ROOT_DIR"

if ! command -v pnpm >/dev/null 2>&1; then
  echo "pnpm not found; attempting to enable via corepack..."
  if command -v corepack >/dev/null 2>&1; then
    corepack enable || true
    corepack prepare pnpm@9.7.1 --activate || true
  fi
fi

command -v pnpm >/dev/null || { echo "pnpm is required. Install via: npm i -g pnpm (or enable corepack)." >&2; exit 1; }

pnpm install --frozen-lockfile
pnpm -C web install --frozen-lockfile

echo "JS dependencies installed."
