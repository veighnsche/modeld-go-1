#!/usr/bin/env bash
set -euo pipefail
go run ./cmd/modeld --config configs/models.yaml --addr :8080
