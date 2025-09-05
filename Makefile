SHELL := /usr/bin/env bash
APP := modeld
BIN := bin/$(APP)
COVER_PROFILE := coverage.out
COVER_MODE := atomic
COVER_THRESHOLD ?= 80
WEB_PORT ?= 5173

.PHONY: build run tidy clean test cover cover-html cover-check e2e-py e2e-py-haiku \
        swagger-install swagger-gen swagger-build swagger-run \
        web-build web-preview web-dev \
        e2e-cy-auto e2e-cy-haiku \
        test-all cli test-cli testctl-build \
        llama-libs build-llama \
        ci-go ci-e2e-python ci-e2e-cypress ci-all

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
	@env -u CGO_LDFLAGS -u CGO_CFLAGS -u LD_LIBRARY_PATH CGO_ENABLED=0 go test ./... -v


cover:
	@pkgs=$$(go list ./... | grep -v '^modeld/cmd/' | grep -v '^modeld/docs$$' | grep -v '^modeld/internal/testctl$$'); \
	env -u CGO_LDFLAGS -u CGO_CFLAGS -u LD_LIBRARY_PATH CGO_ENABLED=0 go test $$pkgs -covermode=$(COVER_MODE) -coverprofile=$(COVER_PROFILE) -v
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

# Run only the Python haiku E2E test through the Go CLI (testctl)
e2e-py-haiku:
	@$(MAKE) testctl-build
	@bin/testctl test py:haiku

# Tooling versions (can be overridden via environment)
SWAG_VERSION ?= v1.16.6
GOLANGCI_LINT_VERSION ?= v1.61.0

# Helpers
# Set FORCE_PORT_UNBLOCK=1 to allow killing listeners; default is non-destructive.
FORCE_PORT_UNBLOCK ?= 0
# Usage: $(call ensure_port_free, <port>)
define ensure_port_free
    if [ "$(FORCE_PORT_UNBLOCK)" = "1" ]; then \
        bash scripts/ports/ensure_ports.sh --force $(1); \
    else \
        bash scripts/ports/ensure_ports.sh $(1); \
    fi;
endef

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
	@$(MAKE) testctl-build
	@bin/testctl install all
	@WEB_PORT=$(WEB_PORT) bin/testctl test all auto

# Build the Go-based test controller CLI
testctl-build:
	@mkdir -p bin
	@go build -o bin/testctl ./cmd/testctl

# Web (Vite + React)
web-build:
	@pnpm -C web build

web-preview:
	@pnpm -C web preview --port $(WEB_PORT)

web-dev:
	@pnpm -C web dev --port $(WEB_PORT)

# llama.cpp integration (no env vars; rpath baked via cgo directives)
# Path where llama.cpp built artifacts reside (bin/ with libllama.so, libggml*.so, llama-server, etc.)
LLAMA_BIN_DIR ?= $(HOME)/src/llama.cpp/build-cuda14/bin

# Copy required shared libs into our local ./bin so rpath '$ORIGIN' resolves at runtime
llama-libs:
 	@mkdir -p bin
 	@test -d "$(LLAMA_BIN_DIR)" || { echo "LLAMA_BIN_DIR not found: $(LLAMA_BIN_DIR)" >&2; exit 1; }
 	@install -m644 $(LLAMA_BIN_DIR)/libllama.so bin/
 	@install -m644 $(LLAMA_BIN_DIR)/libggml*.so bin/ 2>/dev/null || true
 	@echo "Synced llama.cpp shared libs to ./bin"

# Build with the 'llama' build tag and CGO enabled; linking/rpath handled by internal/manager/llama_cgo.go
build-llama: llama-libs
 	@mkdir -p bin
 	@CGO_ENABLED=1 go build -tags=llama -o $(BIN) ./cmd/modeld
 	@echo "Built $(BIN) with llama support (rpath=$$ORIGIN, no env vars required)"

# Cypress E2E (UI harness)
# Keep only the heavy, full-suite target here; all other variants live in the CLI.
e2e-cy-auto:
	@$(MAKE) testctl-build
	@bin/testctl test web auto

# Run only the Haiku cypress spec end-to-end against a live backend using host models
e2e-cy-haiku:
	@$(MAKE) testctl-build
	@WEB_PORT=$(WEB_PORT) bin/testctl test web haiku

# Convenience: launch the Bash CLI helper
cli:
	@$(MAKE) testctl-build
	@bin/testctl

# Run CLI tests
test-cli:
	@bash scripts/tests/cli.test.sh

# CI local runners (via nektos/act)
# Requires: https://github.com/nektos/act (see .actrc for defaults)
ci-all: ci-go ci-e2e-python ci-e2e-cypress ## Run all CI workflows locally (act)

ci-go:
	@command -v act >/dev/null 2>&1 || { echo "act not found. Install: https://github.com/nektos/act" >&2; exit 1; }
	@act -W .github/workflows/ci-go.yml

ci-e2e-python:
	@command -v act >/dev/null 2>&1 || { echo "act not found. Install: https://github.com/nektos/act" >&2; exit 1; }
	@act -W .github/workflows/ci-e2e-python.yml

ci-e2e-cypress:
	@command -v act >/dev/null 2>&1 || { echo "act not found. Install: https://github.com/nektos/act" >&2; exit 1; }
	@act -W .github/workflows/ci-e2e-cypress.yml
