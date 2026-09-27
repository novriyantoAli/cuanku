SHELL := /bin/bash

BIN_DIR := bin
API_BIN := $(BIN_DIR)/cuanku-api
MIGRATION_BIN := $(BIN_DIR)/cuanku-migration

# Every database-touching recipe is wrapped in this. It sources ./.env with the
# *shell* rather than make's own `include`, because a .env file is shell syntax:
# quotes, `$` and line continuations mean what they mean in a shell, and make's
# `include` would interpret `$` as a make variable reference.
#
# NOTE: characters special to URLs are a separate matter — see the encoding note
# in .env.example. `#` in a password still has to be percent-encoded, because
# the DSN parser reads it as the start of a fragment.
#
# Nothing here hardcodes a DSN — the file is git-ignored and a missing value
# fails with instructions instead of connecting somewhere unexpected (AGENTS.md).
define with_dsn
set -a; if [ -f ./.env ]; then . ./.env; fi; set +a; \
if [ -z "$$DATABASE_URL" ]; then \
	echo "DATABASE_URL is not set."; \
	echo "Either put it in backend/.env (git-ignored; see .env.example), or export it:"; \
	echo "  export DATABASE_URL='postgres://user:pass@<vm-address>:5432/<database>?sslmode=disable'"; \
	exit 1; \
fi; \
$(1)
endef

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
run: ## Run the API server (DSN from ./.env or the environment)
	@$(call with_dsn,go run ./cmd/api)

.PHONY: run-migration
run-migration: ## Apply all pending migrations (DSN from ./.env or the environment)
	@$(call with_dsn,go run ./cmd/migration -direction=up)

.PHONY: migrate-up
migrate-up: run-migration ## Alias for run-migration

.PHONY: migrate-down
migrate-down: ## Roll back one migration (DSN from ./.env or the environment)
	@$(call with_dsn,go run ./cmd/migration -direction=down -steps=1)

.PHONY: migrate-version
migrate-version: ## Print the current schema version (DSN from ./.env or the environment)
	@$(call with_dsn,go run ./cmd/migration -direction=version)

.PHONY: test
test: ## Run every test
	go test ./... -count=1

.PHONY: test-unit
test-unit: ## Run unit tests only (database integration tests skip)
	go test ./internal/... ./cmd/... -count=1

.PHONY: test-integration
test-integration: ## Run the database integration tests (DSN from ./.env or the environment)
	@$(call with_dsn,go test ./internal/pkg/migrator/... -count=1 -v -run 'TestUp|TestVersion')

.PHONY: check-db
check-db: ## Prove the DSN reaches PostgreSQL, then report the schema version
	@$(call with_dsn,go run ./cmd/migration -direction=version)

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
docker-run: ## Run the API image (DSN from ./.env or the environment)
	@command -v docker >/dev/null 2>&1 || { echo "docker is not installed on this machine (see AGENTS.md)"; exit 1; }
	@$(call with_dsn,docker run --rm -p $${PORT:-8080}:8080 -e DATABASE_URL -e APP_ENV -e LOGGER_LEVEL cuanku-api:latest)

.PHONY: ci
ci: build vet test lint ## What CI runs
