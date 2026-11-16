# Every way of running the project has a target (docs/architecture/conventions.md).
# Go tools are pinned in go.mod and run with `go tool`.

BINARY        := bin/wallet-accountant
COMPOSE_DEV   := docker compose -f compose.dev.yaml
VACUUM        := go tool vacuum lint --ruleset api/zalando-ruleset.yaml --no-banner --no-style

# Development configuration (compose.dev.yaml). Override any of them on the
# command line or in the environment.
export WA_HTTP_ADDR                 ?= :8080
export WA_DATABASE_URL              ?= postgres://wallet_accountant:wallet_accountant@localhost:5432/wallet_accountant?sslmode=disable
export WA_OIDC_ISSUER               ?= http://localhost:8081
export WA_OIDC_AUDIENCE             ?= wallet-accountant
export OTEL_EXPORTER_OTLP_ENDPOINT  ?= http://localhost:4318
export OTEL_EXPORTER_OTLP_PROTOCOL  ?= http/protobuf

.PHONY: help dev-up dev-down run run-api grafana generate lint lint-go lint-api build test test-integration

help: ## List the targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-18s %s\n", $$1, $$2}'

## Development

dev-up: ## Start PostgreSQL, Zitadel and otel-lgtm
	$(COMPOSE_DEV) up -d --wait

dev-down: ## Stop the development dependencies
	$(COMPOSE_DEV) down

run: ## Run the app in all mode against the dev dependencies
	go run ./cmd/wallet-accountant all

run-api: ## Run the api run mode against the dev dependencies
	go run ./cmd/wallet-accountant api

grafana: ## Open the local Grafana UI
	@open http://localhost:3000 2>/dev/null || xdg-open http://localhost:3000 2>/dev/null || echo "Grafana: http://localhost:3000"

## Build and quality

generate: ## Regenerate code from api/openapi.yaml (ogen)
	go generate ./...

lint: lint-go lint-api ## Run golangci-lint (with depguard) and vacuum with the Zalando ruleset

lint-go:
	go tool golangci-lint run ./...

lint-api:
	$(VACUUM) --details --fail-severity warn api/openapi.yaml
	@# The ruleset must still catch violations: a deliberately broken spec has to fail.
	@if $(VACUUM) --silent api/testdata/zalando-violations.yaml; then \
		echo "vacuum: the Zalando ruleset no longer reports api/testdata/zalando-violations.yaml"; exit 1; \
	fi

build: ## Build the binary
	CGO_ENABLED=0 go build -trimpath -o $(BINARY) ./cmd/wallet-accountant

test: ## Run the unit tests
	go test ./...

test-integration: ## Run the integration tests against PostgreSQL through testcontainers-go
	go test -tags integration -count=1 ./...
