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

# Local cluster (ADR-0019). The local-* targets drive the env repo
# (wallet-accountant-deploy); point ENV_REPO at any checkout of it.
ENV_REPO            ?= ../wallet-accountant-deploy
LOCAL_CLUSTER       ?= wallet-accountant
LOCAL_REGISTRY      ?= wa-registry
LOCAL_REGISTRY_PORT ?= 5050
LOCAL_REGISTRY_DIR  ?= $(HOME)/.local/share/wallet-accountant/registry
# Host ports for Traefik; change them only if something else holds 80 or 443.
LOCAL_HTTP_PORT     ?= 80
LOCAL_HTTPS_PORT    ?= 443
LOCAL_ARTIFACT      := oci://localhost:$(LOCAL_REGISTRY_PORT)/wallet-accountant-deploy:local
LOCAL_KUBECTL       := kubectl --context k3d-$(LOCAL_CLUSTER)
LOCAL_FLUX          := flux --context k3d-$(LOCAL_CLUSTER)
export ENV_REPO LOCAL_CLUSTER LOCAL_REGISTRY LOCAL_REGISTRY_PORT LOCAL_REGISTRY_DIR LOCAL_HTTP_PORT LOCAL_HTTPS_PORT

.PHONY: local-ca local-up local-down local-push local-deploy local-wait local-check local-headlamp-token
.PHONY: help up up-observability down logs bootstrap compose-rebuild smoke docker-build dev-up dev-down dev-reset run run-api run-worker migrate rebuild dev-bootstrap grafana swagger generate lint lint-go lint-api build test test-integration

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

dev-up: ## Start PostgreSQL, Zitadel (with the wallet-accountant project), Swagger UI and otel-lgtm
	$(COMPOSE_DEV) up -d --wait
	$(COMPOSE_DEV) run --rm zitadel-setup
	@# The Bruno collection reads the client ID Zitadel generated from api/bruno/.env.
	@client_id=$$($(COMPOSE_DEV) run --rm --no-deps -T --entrypoint cat zitadel-setup /zitadel-setup/api-clients-client-id.txt) && \
		printf 'WA_OAUTH_CLIENT_ID=%s\n' "$$client_id" > api/bruno/.env && \
		echo "api/bruno/.env: WA_OAUTH_CLIENT_ID=$$client_id"

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

swagger: ## Open Swagger UI on api/openapi.yaml (dev-up starts it)
	@open http://localhost:8082 2>/dev/null || xdg-open http://localhost:8082 2>/dev/null || echo "Swagger UI: http://localhost:8082"

## Local cluster

local-ca: ## Create the mkcert CA if needed and make this Mac trust it (once; asks for your password)
	mkcert -install

local-up: ## Create the k3d cluster with its registry, install Flux and reconcile the env repo's clusters/local
	@test -f "$$(mkcert -CAROOT)/rootCA.pem" || { echo "No mkcert CA yet: run 'make local-ca' once."; exit 1; }
	@test -d "$(ENV_REPO)/clusters/local" || { echo "ENV_REPO=$(ENV_REPO) is not a wallet-accountant-deploy checkout."; exit 1; }
	mkdir -p "$(LOCAL_REGISTRY_DIR)"
	k3d cluster get $(LOCAL_CLUSTER) >/dev/null 2>&1 || \
		k3d cluster create --config "$(ENV_REPO)/clusters/local/k3d.yaml"
	$(LOCAL_FLUX) install
	@# cert-manager's CA issuer signs with this Mac's mkcert CA, whose key never leaves the Mac.
	$(LOCAL_KUBECTL) create namespace cert-manager --dry-run=client -o yaml | $(LOCAL_KUBECTL) apply -f -
	$(LOCAL_KUBECTL) -n cert-manager create secret tls mkcert-ca \
		--cert="$$(mkcert -CAROOT)/rootCA.pem" --key="$$(mkcert -CAROOT)/rootCA-key.pem" \
		--dry-run=client -o yaml | $(LOCAL_KUBECTL) apply -f -
	$(MAKE) --no-print-directory local-push
	$(LOCAL_KUBECTL) apply -k "$(ENV_REPO)/clusters/local/flux-system"
	$(MAKE) --no-print-directory local-wait

local-down: ## Delete the k3d cluster and its registry (the registry's storage is kept)
	k3d cluster delete $(LOCAL_CLUSTER)

local-push:
	@# The env repo's working tree, committed or not, is what Flux applies.
	$(LOCAL_FLUX) push artifact $(LOCAL_ARTIFACT) --path="$(ENV_REPO)" \
		--source="$$(git -C "$(ENV_REPO)" remote get-url origin 2>/dev/null || echo local)" \
		--revision="$$(git -C "$(ENV_REPO)" rev-parse --abbrev-ref HEAD)@sha1:$$(git -C "$(ENV_REPO)" rev-parse HEAD)" \
		--ignore-paths=".git/,infra/"

local-deploy: local-push ## Push the env repo's working tree to the local registry and have Flux apply it
	$(LOCAL_FLUX) reconcile kustomization cluster --with-source --timeout=5m
	$(MAKE) --no-print-directory local-wait

local-wait:
	@# `cluster` creates the other Kustomizations, so they exist only once it is ready.
	$(LOCAL_KUBECTL) -n flux-system wait kustomizations.kustomize.toolkit.fluxcd.io/cluster --for=condition=Ready --timeout=5m
	$(LOCAL_KUBECTL) -n flux-system wait kustomizations.kustomize.toolkit.fluxcd.io --all --for=condition=Ready --timeout=20m

local-check: ## Check the running local cluster against ADR-0019 and the env repo
	deploy/local/check.sh

local-headlamp-token: ## Print a token to sign in to https://headlamp.localhost (valid 24 hours)
	@$(LOCAL_KUBECTL) -n headlamp create token headlamp --duration=24h

## Build and quality

generate: ## Regenerate the ogen server and the Bruno collection (api/bruno) from api/openapi.yaml
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
