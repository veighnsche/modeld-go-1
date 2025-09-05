SHELL := /usr/bin/env bash
APP := modeld
BIN := bin/$(APP)
COVER_PROFILE := coverage.out
COVER_MODE := atomic
COVER_THRESHOLD ?= 80
WEB_PORT ?= 5173

.PHONY: build run tidy clean test cover cover-html cover-check e2e-py \
        swagger-install swagger-gen swagger-build swagger-run \
        web-build web-preview web-dev e2e-cy-mock e2e-cy-live \
        test-all

build:
	@mkdir -p bin
	@go build -o $(BIN) ./cmd/modeld

run:
	@go run ./cmd/modeld

tidy:
	@go mod tidy

clean:
	@rm -rf bin

test:
	@go test ./... -v

cover:
	@pkgs=$$(go list ./... | grep -v '^modeld/cmd/'); \
	go test $$pkgs -covermode=$(COVER_MODE) -coverprofile=$(COVER_PROFILE) -v
	@echo "Coverage profile written to $(COVER_PROFILE)"

cover-html: cover
	@go tool cover -html=$(COVER_PROFILE) -o coverage.html
	@echo "Coverage HTML written to coverage.html"

cover-check: cover
	@percent=$(shell go tool cover -func=$(COVER_PROFILE) | awk '/^total:/ {gsub("%","",$$3); print $$3}') ; \
	awk -v p="$$percent" -v t="$(COVER_THRESHOLD)" 'BEGIN { if (p+0 < t+0) { printf("Coverage %.2f%% is below threshold %d%%\n", p, t); exit 1 } else { printf("Coverage %.2f%% meets threshold %d%%\n", p, t); exit 0 } }'

e2e-py:
	@python3 -m venv .venv
	@source .venv/bin/activate && pip install -r tests/e2e_py/requirements.txt && pytest -q tests/e2e_py

# Tooling versions (can be overridden via environment)
SWAG_VERSION ?= v1.16.6
GOLANGCI_LINT_VERSION ?= v1.56.2

# Swagger / OpenAPI helpers
swagger-install:
	@go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

swagger-gen:
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/modeld/main.go -o docs ; \
	else \
		go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION) init -g cmd/modeld/main.go -o docs ; \
	fi

swagger-build:
	@mkdir -p bin
	@go build -tags=swagger -o $(BIN) ./cmd/modeld

swagger-run:
	@go run -tags=swagger ./cmd/modeld

# Linting
install-golangci-lint:
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found; run 'make install-golangci-lint'" >&2; exit 1; }
	@golangci-lint run

test-all:
	@bash scripts/tests/all.sh

# Web (Vite + React)
web-build:
	@pnpm -C web build

web-preview:
	@pnpm -C web preview --port $(WEB_PORT)

web-dev:
	@pnpm -C web dev --port $(WEB_PORT)

# Cypress E2E (UI harness)
# Mock mode: serves built web with Vite preview and runs Cypress without a live API.
e2e-cy-mock:
	@set -euo pipefail; \
	pnpm -C web build; \
	pnpm -C web preview --port $(WEB_PORT) & \
	PREVIEW_PID=$$!; \
	trap 'kill $$PREVIEW_PID || true' EXIT; \
	node scripts/cli/poll-url.js http://localhost:$(WEB_PORT) 200 60 || true; \
	CYPRESS_BASE_URL=http://localhost:$(WEB_PORT) CYPRESS_USE_MOCKS=1 pnpm run test:e2e:run

# Live mode: additionally starts the Go API with a temporary models dir.
e2e-cy-live:
	@set -euo pipefail; \
	pnpm -C web build; \
	pnpm -C web preview --port $(WEB_PORT) & \
	PREVIEW_PID=$$!; \
	trap 'kill $$PREVIEW_PID || true' EXIT; \
	node scripts/cli/poll-url.js http://localhost:$(WEB_PORT) 200 60 || true; \
	mkdir -p models_tmp; \
	touch models_tmp/alpha.gguf models_tmp/beta.gguf; \
	(go run ./cmd/modeld --addr :18080 --models-dir $$(pwd)/models_tmp --default-model alpha.gguf &) ; \
	node scripts/cli/poll-url.js http://localhost:18080/healthz 200 60 || true; \
	CYPRESS_BASE_URL=http://localhost:$(WEB_PORT) \
	CYPRESS_API_HEALTH_URL=http://localhost:18080/healthz \
	CYPRESS_API_READY_URL=http://localhost:18080/readyz \
	CYPRESS_API_STATUS_URL=http://localhost:18080/status \
	CYPRESS_USE_MOCKS=0 pnpm run test:e2e:run
