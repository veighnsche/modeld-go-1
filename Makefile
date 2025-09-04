SHELL := /usr/bin/env bash
APP := modeld
BIN := bin/$(APP)
COVER_PROFILE := coverage.out
COVER_MODE := atomic
COVER_THRESHOLD ?= 80

.PHONY: build run tidy clean test cover cover-html cover-check e2e-py \
        swagger-install swagger-gen swagger-build swagger-run

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

# Swagger / OpenAPI helpers
swagger-install:
	@go install github.com/swaggo/swag/cmd/swag@latest

swagger-gen:
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/modeld/main.go -o docs ; \
	else \
		go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/modeld/main.go -o docs ; \
	fi

swagger-build:
	@mkdir -p bin
	@go build -tags=swagger -o $(BIN) ./cmd/modeld

swagger-run:
	@go run -tags=swagger ./cmd/modeld
