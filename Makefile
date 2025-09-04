SHELL := /usr/bin/env bash
APP := modeld
BIN := bin/$(APP)

.PHONY: build run tidy clean
build:
	@mkdir -p bin
	@go build -o $(BIN) ./cmd/modeld

run:
	@go run ./cmd/modeld

tidy:
	@go mod tidy

clean:
	@rm -rf bin
