# Every way of running the project has a target (docs/architecture/conventions.md).
# Go tools are pinned in go.mod and run with `go tool`.

BINARY        := bin/wallet-accountant
COMPOSE_DEV   := docker compose -f compose.dev.yaml
IMAGE         ?= wallet-accountant:local

# Self-hosting (compose.yaml). ENV_FILE holds the secrets; `make up` writes it
# on first use. Set COMPOSE_PROJECT_NAME or COMPOSE_FILE to run another
# installation side by side (deploy/selfhosted/smoke.sh does).
ENV_FILE      ?= .env
COMPOSE       := docker compose --env-file $(ENV_FILE)
OBSERVABILITY := COLLECTOR_CONFIG=observability $(COMPOSE) --profile observability
VACUUM        := go tool vacuum lint --ruleset api/zalando-ruleset.yaml --no-banner --no-style

# Development configuration (compose.dev.yaml). Override any of them on the
# command line or in the environment.
export WA_HTTP_ADDR                 ?= :8080
export WA_DATABASE_URL              ?= postgres://wa_app:wa_app@localhost:5432/wallet_accountant?sslmode=disable
export WA_READER_DATABASE_URL       ?= postgres://wa_event_store_reader:wa_event_store_reader@localhost:5432/wallet_accountant?sslmode=disable
export WA_OWNER_DATABASE_URL        ?= postgres://wa_owner:wa_owner@localhost:5432/wallet_accountant?sslmode=disable
export WA_OIDC_ISSUER               ?= http://localhost:8081
export WA_OIDC_AUDIENCE             ?= wallet-accountant
export OTEL_EXPORTER_OTLP_ENDPOINT  ?= http://localhost:4318
export OTEL_EXPORTER_OTLP_PROTOCOL  ?= http/protobuf

.PHONY: help up up-observability down logs bootstrap compose-rebuild smoke docker-build dev-up dev-down dev-reset run run-api run-worker migrate rebuild dev-bootstrap grafana generate lint lint-go lint-api build test test-integration

help: ## List the targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-18s %s\n", $$1, $$2}'

## Self-hosted

$(ENV_FILE):
	deploy/selfhosted/init-env.sh $(ENV_FILE)

up: $(ENV_FILE) ## Start the app, migrations, PostgreSQL, Zitadel and the collector
	$(COMPOSE) up -d --build --wait

up-observability: $(ENV_FILE) ## Same as up, plus Tempo, Prometheus, Loki and Grafana
	$(OBSERVABILITY) up -d --build --wait

down: ## Stop the self-hosted stack (the volumes are kept)
	$(COMPOSE) --profile observability down

logs: ## Follow the app's logs
	$(COMPOSE) logs -f app

bootstrap: ## Print the bootstrap Invitation code for the first Instance administrator (ADR-0017)
	@# The code goes only to this terminal: the container is removed, and the
	@# app logs no more than that an Invitation was issued.
	@$(COMPOSE) run --rm --no-deps app bootstrap

compose-rebuild: ## Rebuild every fold from the event store in the self-hosted database
	$(COMPOSE) run --rm --no-deps app rebuild

smoke: ## From scratch, run up and bootstrap in a separate Compose project and redeem the code
	deploy/selfhosted/smoke.sh

## Development

dev-up: ## Start PostgreSQL, Zitadel (with the wallet-accountant project) and otel-lgtm
	$(COMPOSE_DEV) up -d --wait
	$(COMPOSE_DEV) run --rm zitadel-setup

dev-down: ## Stop the development dependencies (the volumes are kept)
	$(COMPOSE_DEV) down

dev-reset: ## Delete the dev database and Zitadel data, then run dev-up and migrate
	$(COMPOSE_DEV) --profile setup down -v --remove-orphans
	$(MAKE) --no-print-directory dev-up migrate

run: ## Run the app in all mode against the dev dependencies
	go run ./cmd/wallet-accountant all

run-api: ## Run the api run mode against the dev dependencies
	go run ./cmd/wallet-accountant api

run-worker: ## Run the worker run mode (event forwarding) against the dev dependencies
	go run ./cmd/wallet-accountant worker

migrate: ## Run the migrate run mode against the dev database
	go run ./cmd/wallet-accountant migrate

rebuild: ## Rebuild every fold from the event store in the dev database
	go run ./cmd/wallet-accountant rebuild

dev-bootstrap: ## Print the bootstrap Invitation code for the first Instance administrator (dev database)
	go run ./cmd/wallet-accountant bootstrap

grafana: ## Open the local Grafana UI
	@open http://localhost:3000 2>/dev/null || xdg-open http://localhost:3000 2>/dev/null || echo "Grafana: http://localhost:3000"

## Build and quality

generate: ## Regenerate code from api/openapi.yaml (ogen)
	go generate ./...

lint: lint-go lint-api ## Run golangci-lint (with depguard) and vacuum with the Zalando ruleset

lint-go:
	go tool golangci-lint run ./...

lint-api:
	@# A tree (the Category tree) recurses through an array, which always ends.
	$(VACUUM) --details --fail-severity warn --ignore-array-circle-ref api/openapi.yaml
	@# The ruleset must still catch violations: a deliberately broken spec has to fail.
	@if $(VACUUM) --silent api/testdata/zalando-violations.yaml; then \
		echo "vacuum: the Zalando ruleset no longer reports api/testdata/zalando-violations.yaml"; exit 1; \
	fi

build: ## Build the binary
	CGO_ENABLED=0 go build -trimpath -o $(BINARY) ./cmd/wallet-accountant

docker-build: ## Build the container image
	docker build -t $(IMAGE) .

test: ## Run the unit tests
	go test ./...

test-integration: ## Run the integration tests against PostgreSQL through testcontainers-go
	go test -tags integration -count=1 ./...
