SHELL := /bin/bash

BIN_DIR := bin
API_BIN := $(BIN_DIR)/cuanku-api
MIGRATION_BIN := $(BIN_DIR)/cuanku-migration

# Never hardcode a DSN here: this guard fails fast with a useful message
# instead of silently connecting somewhere unexpected (AGENTS.md).
require-dsn:
	@if [ -z "$$DATABASE_URL" ]; then \
		echo "DATABASE_URL is not set. Export the dev DSN first, e.g."; \
		echo "  export DATABASE_URL='postgres://user:pass@<vm-host>:5432/cuanku?sslmode=disable'"; \
		exit 1; \
	fi

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: deps
deps: ## Download and tidy module dependencies
	go mod download
	go mod tidy

.PHONY: build
build: build-api build-migration ## Build every binary

.PHONY: build-api
build-api: ## Build the HTTP API binary
	go build -trimpath -o $(API_BIN) ./cmd/api

.PHONY: build-migration
build-migration: ## Build the migration runner binary
	go build -trimpath -o $(MIGRATION_BIN) ./cmd/migration

.PHONY: run
run: ## Run the API server (requires DATABASE_URL)
	$(MAKE) require-dsn
	go run ./cmd/api

.PHONY: run-migration
run-migration: ## Apply all pending migrations (requires DATABASE_URL)
	$(MAKE) require-dsn
	go run ./cmd/migration -direction=up

.PHONY: migrate-up
migrate-up: run-migration ## Alias for run-migration

.PHONY: migrate-down
migrate-down: ## Roll back one migration (requires DATABASE_URL)
	$(MAKE) require-dsn
	go run ./cmd/migration -direction=down -steps=1

.PHONY: migrate-version
migrate-version: ## Print the current schema version (requires DATABASE_URL)
	$(MAKE) require-dsn
	go run ./cmd/migration -direction=version

.PHONY: test
test: ## Run every test
	go test ./... -count=1

.PHONY: test-unit
test-unit: ## Run unit tests only
	go test ./internal/... ./cmd/... -count=1

.PHONY: test-coverage
test-coverage: ## Run tests with a coverage report
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with fixes
	golangci-lint run --fix ./...

.PHONY: format
format: ## Format with gofmt + goimports
	gofmt -w .
	goimports -w -local github.com/novriyantoAli/cuanku/backend . 2>/dev/null || true

.PHONY: format-strict
format-strict: ## Format with gofumpt (installed separately)
	@command -v gofumpt >/dev/null 2>&1 || { echo "gofumpt not installed: go install mvdan.cc/gofumpt@latest"; exit 1; }
	gofumpt -w .

.PHONY: docker-build
docker-build: ## Build the API image (needs a container runtime; none on the dev machine)
	@command -v docker >/dev/null 2>&1 || { echo "docker is not installed on this machine (see AGENTS.md) — build the image elsewhere"; exit 1; }
	docker build -t cuanku-api:$(shell git rev-parse --short HEAD) .

.PHONY: docker-run
docker-run: ## Run the API image (requires DATABASE_URL in the environment)
	$(MAKE) require-dsn
	@command -v docker >/dev/null 2>&1 || { echo "docker is not installed on this machine (see AGENTS.md)"; exit 1; }
	docker run --rm -p $(or $(PORT),8080):8080 -e DATABASE_URL -e APP_ENV -e LOGGER_LEVEL cuanku-api:latest

.PHONY: ci
ci: build vet test lint ## What CI runs
