SHELL := /usr/bin/env bash
APP := modeld
BIN := bin/$(APP)
COVER_PROFILE := coverage.out
COVER_MODE := atomic
COVER_THRESHOLD ?= 80

.PHONY: build run tidy clean test cover cover-html cover-check

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
	@go test ./... -covermode=$(COVER_MODE) -coverprofile=$(COVER_PROFILE) -v
	@echo "Coverage profile written to $(COVER_PROFILE)"

cover-html: cover
	@go tool cover -html=$(COVER_PROFILE) -o coverage.html
	@echo "Coverage HTML written to coverage.html"

cover-check: cover
	@percent=$(shell go tool cover -func=$(COVER_PROFILE) | awk '/^total:/ {gsub("%","",$$3); print $$3}') ; \
	awk -v p="$$percent" -v t="$(COVER_THRESHOLD)" 'BEGIN { if (p+0 < t+0) { printf("Coverage %.2f%% is below threshold %d%%\n", p, t); exit 1 } else { printf("Coverage %.2f%% meets threshold %d%%\n", p, t); exit 0 } }'
