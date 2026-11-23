# Every way of running the project has a target (docs/architecture/conventions.md).
# Go tools are pinned in go.mod and run with `go tool`.

BINARY        := bin/wallet-accountant
IMAGE         ?= wallet-accountant:local
VACUUM        := go tool vacuum lint --ruleset api/zalando-ruleset.yaml --no-banner --no-style

# The Helm chart's checks (deploy/helm/wallet-accountant). helm and kubeconform
# are built into bin/ at pinned versions, not go.mod tools: their dependencies
# would join the app's module graph. kubeconform reads the CloudNativePG and
# Gateway API schemas from the CRDs catalog at a pinned commit.
CHART               := deploy/helm/wallet-accountant
# The `# renovate:` comments let Renovate bump these (renovate.json).
# renovate: datasource=go depName=helm.sh/helm/v4
HELM_VERSION        := v4.3.2
# renovate: datasource=go depName=github.com/yannh/kubeconform
KUBECONFORM_VERSION := v0.8.0
HELM                := bin/helm-$(HELM_VERSION)/helm
KUBECONFORM         := bin/kubeconform-$(KUBECONFORM_VERSION)/kubeconform
CHART_REGISTRY      ?= oci://ghcr.io/mumia/charts
# actionlint checks .github/workflows; run with `go run`, not a go.mod tool,
# for the same reason as helm.
ACTIONLINT_VERSION  := v1.7.12
CRDS_CATALOG        := https://raw.githubusercontent.com/datreeio/CRDs-catalog/784ec895547a36e548e6810b41f27baade3a0890/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json

# Local cluster (ADR-0019). The local-* targets drive the env repo
# (wallet-accountant-deploy); point ENV_REPO at any checkout of it.
ENV_REPO            ?= ../wallet-accountant-deploy
LOCAL_CLUSTER       ?= wallet-accountant
LOCAL_REGISTRY      ?= wa-registry
LOCAL_REGISTRY_PORT ?= 5050
LOCAL_REGISTRY_DIR  ?= $(HOME)/.local/share/wallet-accountant/registry
# The registry's configuration file, which k3d mounts (an absolute path).
LOCAL_REGISTRY_CONFIG ?= $(abspath $(ENV_REPO))/clusters/local/registry-config.yml
# Host ports for Traefik, so another cluster on the Mac can keep 80 and 443.
# Browser URLs are https://<name>.localhost:18443. Zitadel's issuer and the
# in-cluster port that pods reach it on are 18443 too (env repo,
# clusters/local): change those with LOCAL_HTTPS_PORT, or sign-in breaks.
LOCAL_HTTP_PORT     ?= 18080
LOCAL_HTTPS_PORT    ?= 18443
# Debug mode: apps-db and the app's collector on this Mac's localhost.
LOCAL_DB_PORT       ?= 5432
LOCAL_OTLP_PORT     ?= 4318
# The Grafana stack (ADR-0016) is opt-in locally: `make local-up-observability`,
# or OBSERVABILITY=true on local-up and local-check.
OBSERVABILITY       ?= false
# The k3s API server trusts this Mac's mkcert CA to verify Zitadel's tokens
# (Headlamp's single sign-on); k3d mounts it into the node.
LOCAL_MKCERT_CA     ?= $(shell mkcert -CAROOT 2>/dev/null)/rootCA.pem
# k3d mounts files from the env repo, and needs an absolute path for that.
LOCAL_ENV_REPO_DIR   = $(abspath $(ENV_REPO))
LOCAL_ARTIFACT      := oci://localhost:$(LOCAL_REGISTRY_PORT)/wallet-accountant-deploy:local
LOCAL_KUBECTL       := kubectl --context k3d-$(LOCAL_CLUSTER)
LOCAL_FLUX          := flux --context k3d-$(LOCAL_CLUSTER)
# The last build `make local-deploy` pushed (image tag and chart version), and
# the copy of the env repo local-push stages for the artifact.
LOCAL_BUILD         := bin/local-build.env
# The real UI's last build, which wallet-accountant-ui's `make local-deploy`
# records (its image in the registry and tag); local-push deploys it with the
# app, or the stand-in UI when that build is newer (deploy/local/stage.sh).
LOCAL_UI_BUILD      ?= ../wallet-accountant-ui/bin/local-build.env
LOCAL_STAGE         := bin/local-artifact
LOCAL_ORIGIN        := $(if $(filter 443,$(LOCAL_HTTPS_PORT)),,:$(LOCAL_HTTPS_PORT))
# The stand-in UI (spec "Testing Decisions"): a test fixture meeting the UI
# contract, deployed by `make local-deploy LOCAL_STAND_IN_UI=1` and driven by
# a Playwright test in the official image (no Node.js on the host). The
# image's tag must equal @playwright/test's version in $(STAND_IN_UI)/e2e.
STAND_IN_UI         := $(CHART)/test/stand-in-ui
LOCAL_STAND_IN_UI   ?=
# renovate: datasource=docker depName=mcr.microsoft.com/playwright
PLAYWRIGHT_VERSION  := v1.65.1-noble
PLAYWRIGHT_IMAGE    := mcr.microsoft.com/playwright:$(PLAYWRIGHT_VERSION)
export ENV_REPO LOCAL_CLUSTER LOCAL_REGISTRY LOCAL_REGISTRY_PORT LOCAL_REGISTRY_DIR LOCAL_REGISTRY_CONFIG LOCAL_HTTP_PORT LOCAL_HTTPS_PORT OBSERVABILITY LOCAL_MKCERT_CA LOCAL_ENV_REPO_DIR
export LOCAL_DB_PORT LOCAL_OTLP_PORT LOCAL_BUILD LOCAL_UI_BUILD STAND_IN_UI PLAYWRIGHT_IMAGE

# Production (the env repo's clusters/prod), reached over Tailscale with the
# admin kubeconfig infra/homelab writes: `tofu output -raw kubeconfig >
# ~/.kube/shire.yaml` (the env repo's infra/README.md, step 5).
PROD_KUBECONFIG     ?= $(HOME)/.kube/shire.yaml
PROD_CONTEXT        ?= shire

# Debug mode (ADR-0019): `make run` and the other run targets, or an IDE with
# the same variables, run the app on this Mac against the local cluster's
# apps-db, Zitadel and collector. The passwords are the env repo's throwaway
# ones (clusters/local/tenants/wallet-accountant/secrets.yaml). Go trusts the
# mkcert CA through the macOS keychain (`make local-ca`). Override any of them
# on the command line or in the environment.
LOCAL_DB                            := localhost:$(LOCAL_DB_PORT)/wallet_accountant?sslmode=require
export WA_HTTP_ADDR                 ?= :8080
export WA_DATABASE_URL              ?= postgres://wa_app:throwaway-local-wa-app@$(LOCAL_DB)
export WA_READER_DATABASE_URL       ?= postgres://wa_event_store_reader:throwaway-local-wa-event-store-reader@$(LOCAL_DB)
export WA_OWNER_DATABASE_URL        ?= postgres://wa_owner:throwaway-local-wa-owner@$(LOCAL_DB)
export WA_OIDC_ISSUER               ?= https://auth.localhost$(LOCAL_ORIGIN)
export WA_OIDC_AUDIENCE             ?= wallet-accountant
export OTEL_EXPORTER_OTLP_ENDPOINT  ?= http://localhost:$(LOCAL_OTLP_PORT)
export OTEL_EXPORTER_OTLP_PROTOCOL  ?= http/protobuf

.PHONY: renovate-check
.PHONY: local-ca local-up local-up-observability local-down local-build local-push local-deploy local-wait local-check local-headlamp-token
.PHONY: local-bootstrap local-rebuild local-bruno-env local-prefetch images-lock-check images-lock-pin flux-version local-browser-test local-smoke stand-in-ui-vendor
.PHONY: prod-check prod-bootstrap prod-bruno-env
.PHONY: help docker-build run run-api run-worker migrate rebuild dev-bootstrap generate lint lint-go lint-api lint-chart lint-workflows chart-package chart-push print-helm build test test-integration

help: ## List the targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-18s %s\n", $$1, $$2}'

## Debug mode

run: ## Debug mode: run the app in all mode on this Mac against the local cluster's apps-db, Zitadel and collector
	go run ./cmd/wallet-accountant all

run-api: ## Debug mode: run the api run mode against the local cluster
	go run ./cmd/wallet-accountant api

run-worker: ## Debug mode: run the worker run mode (event forwarding) against the local cluster
	go run ./cmd/wallet-accountant worker

migrate: ## Debug mode: run the migrate run mode against the local cluster's apps-db
	go run ./cmd/wallet-accountant migrate

rebuild: ## Debug mode: rebuild every fold from the event store in the local cluster's apps-db
	go run ./cmd/wallet-accountant rebuild

dev-bootstrap: ## Debug mode: print a bootstrap Invitation code (local cluster's apps-db)
	go run ./cmd/wallet-accountant bootstrap

## Local cluster

local-ca: ## Create the mkcert CA if needed and make this Mac trust it (once; asks for your password)
	mkcert -install

local-prefetch: $(HELM) ## Online, once: mirror every image and chart in ENV_REPO's images.lock into the cluster's registry and Docker, so local-up and local-deploy then work offline
	HELM=$(abspath $(HELM)) deploy/local/prefetch.sh

local-up: ## Create the k3d cluster with its registry, install Flux and reconcile the env repo's clusters/local
	@test -f "$$(mkcert -CAROOT)/rootCA.pem" || { echo "No mkcert CA yet: run 'make local-ca' once."; exit 1; }
	@test -d "$(ENV_REPO)/clusters/local" || { echo "ENV_REPO=$(ENV_REPO) is not a wallet-accountant-deploy checkout."; exit 1; }
	mkdir -p "$(LOCAL_REGISTRY_DIR)"
	@# Offline once everything is in: it then only checks what is already there.
	$(MAKE) --no-print-directory local-prefetch
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
	@# The Grafana stack is its own Flux entry point, outside `cluster`, so
	@# applying or deleting it is the switch; deleting prunes the stack.
	@if [ "$(OBSERVABILITY)" = true ]; then \
		$(LOCAL_KUBECTL) apply -f "$(ENV_REPO)/clusters/local/flux-system/observability.yaml"; \
	else \
		$(LOCAL_KUBECTL) -n flux-system delete kustomizations.kustomize.toolkit.fluxcd.io observability --ignore-not-found; \
	fi
	@# Not the app: its chart is the last one local-deploy built, which may not
	@# fit this env repo (a values change); local-deploy builds it and waits on it.
	$(MAKE) --no-print-directory local-wait LOCAL_WAIT_SKIP=wallet-accountant
	@# The app reads the flag (the ConfigMap flux-system/observability) when its
	@# Flux Kustomization reconciles, which may have been before the flag
	@# changed: ask for a reconcile, without waiting on it.
	@if $(LOCAL_KUBECTL) -n flux-system get kustomizations.kustomize.toolkit.fluxcd.io wallet-accountant >/dev/null 2>&1; then \
		$(LOCAL_KUBECTL) -n flux-system annotate --overwrite kustomizations.kustomize.toolkit.fluxcd.io wallet-accountant \
			reconcile.fluxcd.io/requestedAt="$$(date +%s)"; \
	fi

local-up-observability: ## Same as local-up, plus the Grafana stack at https://grafana.localhost:18443
	$(MAKE) --no-print-directory local-up OBSERVABILITY=true

local-down: ## Delete the k3d cluster and its registry (the registry's storage is kept)
	k3d cluster delete $(LOCAL_CLUSTER)
	@# The temporary registry an interrupted local-prefetch may have left on the registry's port.
	docker rm -f $(LOCAL_REGISTRY)-prefetch >/dev/null 2>&1 || true

local-push:
	@# The env repo's working tree, committed or not, is what Flux applies, with
	@# this Mac's CA, the API contract and the last build (deploy/local/stage.sh).
	deploy/local/stage.sh "$(LOCAL_STAGE)"
	$(LOCAL_FLUX) push artifact $(LOCAL_ARTIFACT) --path="$(LOCAL_STAGE)" \
		--source="$$(git -C "$(ENV_REPO)" remote get-url origin 2>/dev/null || echo local)" \
		--revision="$$(git -C "$(ENV_REPO)" rev-parse --abbrev-ref HEAD)@sha1:$$(git -C "$(ENV_REPO)" rev-parse HEAD)"

local-build: $(HELM)
	@# A unique tag per build, so Flux rolls each one out. The image and the chart
	@# come from this working tree, committed or not. Offline: the modules come
	@# from the module cache (local-prefetch filled it), vendored into bin/vendor
	@# (-e: a go.mod tool's package may not load; the app's do), and the build
	@# has no network (Dockerfile). Every command of the recipe below is its own
	@# under set -e (never `a && b`, which set -e lets fail), so a failed build
	@# or push stops it before it records a tag.
	rm -rf bin/vendor
	GOFLAGS=-mod=mod GOPROXY=off go mod vendor -e -o bin/vendor
	@set -e; stamp=$$(date -u +%Y%m%d%H%M%S); tag=local-$$stamp; \
	version=$$(awk '$$1 == "version:" {print $$2}' $(CHART)/Chart.yaml)-local.$$stamp; \
	image=localhost:$(LOCAL_REGISTRY_PORT)/wallet-accountant:$$tag; \
	docker build --network=none --build-context vendor=bin/vendor -t $$image .; \
	docker push -q $$image; \
	rm -rf bin/local-chart; \
	$(HELM) package $(CHART) --destination bin/local-chart --version $$version --app-version $$tag; \
	$(HELM) push bin/local-chart/wallet-accountant-$$version.tgz oci://localhost:$(LOCAL_REGISTRY_PORT)/charts --plain-http; \
	printf 'WALLET_ACCOUNTANT_VERSION=%s\nWALLET_ACCOUNTANT_IMAGE_TAG=%s\n' $$version $$tag > $(LOCAL_BUILD); \
	echo "local-build: image $$image, chart $$version"; \
	if [ -n "$(LOCAL_STAND_IN_UI)" ]; then \
		ui=localhost:$(LOCAL_REGISTRY_PORT)/wallet-accountant-stand-in-ui:$$tag; \
		docker build -t $$ui $(STAND_IN_UI); \
		docker push -q $$ui; \
		printf 'WALLET_ACCOUNTANT_UI_IMAGE=wallet-accountant-stand-in-ui\nWALLET_ACCOUNTANT_UI_TAG=%s\n' $$tag >> $(LOCAL_BUILD); \
		echo "local-build: stand-in UI $$ui"; \
	fi

local-deploy: local-build local-push ## Build the app's image and chart from this working tree, push them with the env repo's working tree, and have Flux roll them out (LOCAL_STAND_IN_UI=1 adds the stand-in UI)
	$(LOCAL_FLUX) reconcile kustomization cluster --with-source --timeout=5m
	$(LOCAL_FLUX) reconcile kustomization wallet-accountant --timeout=20m
	$(MAKE) --no-print-directory local-wait local-bruno-env

local-bruno-env:
	@# The Bruno collection reads the client ID the setup Job generated from
	@# api/bruno/.env; production's (prod-bruno-env) is kept.
	@client_id=$$($(LOCAL_KUBECTL) -n wallet-accountant get configmap wallet-accountant-oidc -o jsonpath='{.data.api-clients-client-id}') && \
		test -n "$$client_id" && touch api/bruno/.env && \
		{ grep -v '^WA_OAUTH_CLIENT_ID=' api/bruno/.env; printf 'WA_OAUTH_CLIENT_ID=%s\n' "$$client_id"; } > api/bruno/.env.new && \
		mv api/bruno/.env.new api/bruno/.env && \
		echo "api/bruno/.env: WA_OAUTH_CLIENT_ID=$$client_id"

local-bootstrap: $(HELM) ## Print a bootstrap Invitation code for the first Instance administrator (local cluster; ADR-0017)
	@# The code goes only to this terminal: the pod and its log are deleted.
	@HELM=$(abspath $(HELM)) deploy/local/operator-pod.sh bootstrap

local-rebuild: $(HELM) ## Rebuild every fold from the event store in the local cluster
	@HELM=$(abspath $(HELM)) deploy/local/operator-pod.sh rebuild

local-wait:
	@# `cluster` creates the other Kustomizations, so they exist only once it is
	@# ready. Then every one but those LOCAL_WAIT_SKIP names.
	$(LOCAL_KUBECTL) -n flux-system wait kustomizations.kustomize.toolkit.fluxcd.io/cluster --for=condition=Ready --timeout=5m
	@set -e; names=$$($(LOCAL_KUBECTL) -n flux-system get kustomizations.kustomize.toolkit.fluxcd.io -o name | \
		grep -v -x -F -e '' $(foreach k,$(LOCAL_WAIT_SKIP),-e kustomization.kustomize.toolkit.fluxcd.io/$(k))); \
	echo "local-wait: $$names" | tr '\n' ' '; echo; \
	$(LOCAL_KUBECTL) -n flux-system wait $$names --for=condition=Ready --timeout=20m

local-check: $(HELM) ## Check the running local cluster against ADR-0019 and the env repo (OBSERVABILITY=true: with the Grafana stack)
	HELM=$(abspath $(HELM)) deploy/local/check.sh

# Offline (ADR-0019): images.lock in ENV_REPO lists every image and chart the
# local cluster uses. CI runs images-lock-check (ci.yaml, the images-lock job).
IMAGESLOCK := go run ./deploy/local/imageslock
# Dockerfiles whose base images images.lock must list (the Mac's Docker builds from them).
LOCK_DOCKERFILES := Dockerfile $(STAND_IN_UI)/Dockerfile

images-lock-check: $(HELM) ## Fail when what clusters/local renders to (manifests, charts with the env repo's values, Flux, k3d and its k3s release's own images, Dockerfiles) uses an image or chart missing from ENV_REPO's images.lock, or the flux CLI is not flux-version
	$(IMAGESLOCK) check -env "$(ENV_REPO)" -helm $(HELM) -chart $(CHART) $(foreach f,$(LOCK_DOCKERFILES),-dockerfile $(f))

flux-version: ## Print the Flux release ENV_REPO pins (infra/homelab's flux_version): the flux CLI every check, CI and the local cluster use
	@$(IMAGESLOCK) flux-version -env "$(ENV_REPO)"

images-lock-pin: ## Add the digest to every image in ENV_REPO's images.lock that has none (online)
	$(IMAGESLOCK) pin -lock "$(ENV_REPO)/images.lock"

local-browser-test: ## Run the stand-in UI's Playwright test (in Docker) against a cluster deployed with LOCAL_STAND_IN_UI=1
	@# Traces and screenshots of a failed run land in bin/stand-in-ui-results.
	RESULTS_DIR=bin/stand-in-ui-results deploy/local/browser-test.sh

local-smoke: ## From nothing: local-up, deploy with the stand-in UI, bootstrap, redeem as a Zitadel identity, the browser test, then local-down
	deploy/local/smoke.sh

stand-in-ui-vendor: ## Copy oidc-client-ts's browser bundle, at the version the stand-in UI's e2e/package.json names, into its site/vendor (in Docker)
	docker run --rm --user "$$(id -u):$$(id -g)" -e HOME=/tmp -v "$(abspath $(STAND_IN_UI)):/src" -w /tmp $(PLAYWRIGHT_IMAGE) sh -c '\
		cp /src/e2e/package.json /src/e2e/package-lock.json . && npm ci --no-audit --no-fund --loglevel=error && \
		cp node_modules/oidc-client-ts/dist/browser/oidc-client-ts.min.js /src/site/vendor/ && \
		cp node_modules/oidc-client-ts/LICENSE /src/site/vendor/oidc-client-ts.LICENSE'

local-headlamp-token: ## Print a token to sign in to https://headlamp.localhost:18443 (valid 24 hours)
	@$(LOCAL_KUBECTL) -n headlamp create token headlamp --duration=24h

## Production

prod-check: $(HELM) ## Build and check the env repo's clusters/prod without a cluster, and this chart with its values (REQUIRE_ENCRYPTED=1: every Secret sealed)
	WALLET_ACCOUNTANT_CHART=$(abspath $(CHART)) HELM=$(abspath $(HELM)) "$(ENV_REPO)/clusters/prod/check.sh"

prod-bootstrap: $(HELM) ## Print a bootstrap Invitation code for production's first Instance administrator (ADR-0017), over Tailscale
	@test -f "$(PROD_KUBECONFIG)" || { echo "No $(PROD_KUBECONFIG): write it as the env repo's infra/README.md (step 5) says."; exit 1; }
	@# The code goes only to this terminal: the pod and its log are deleted.
	@KUBECONFIG="$(PROD_KUBECONFIG)" KUBE_CONTEXT=$(PROD_CONTEXT) CHART_REPO=$(CHART_REGISTRY)/wallet-accountant \
		HELM=$(abspath $(HELM)) deploy/local/operator-pod.sh bootstrap

prod-bruno-env: ## Write production's api-clients client ID to api/bruno/.env (WA_PROD_OAUTH_CLIENT_ID), for Bruno's environment prod
	@test -f "$(PROD_KUBECONFIG)" || { echo "No $(PROD_KUBECONFIG): write it as the env repo's infra/README.md (step 5) says."; exit 1; }
	@client_id=$$(kubectl --kubeconfig "$(PROD_KUBECONFIG)" --context $(PROD_CONTEXT) -n wallet-accountant get configmap wallet-accountant-oidc -o jsonpath='{.data.api-clients-client-id}') && \
		test -n "$$client_id" && touch api/bruno/.env && \
		{ grep -v '^WA_PROD_OAUTH_CLIENT_ID=' api/bruno/.env; printf 'WA_PROD_OAUTH_CLIENT_ID=%s\n' "$$client_id"; } > api/bruno/.env.new && \
		mv api/bruno/.env.new api/bruno/.env && \
		echo "api/bruno/.env: WA_PROD_OAUTH_CLIENT_ID=$$client_id"

## Build and quality

generate: ## Regenerate the ogen server and the Bruno collection (api/bruno) from api/openapi.yaml
	go generate ./...

lint: lint-go lint-api lint-chart lint-workflows ## Run golangci-lint (with depguard), vacuum with the Zalando ruleset, the chart and workflow checks

lint-workflows: ## Check .github/workflows with actionlint (and shellcheck, when it is installed)
	go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

lint-chart: $(HELM) $(KUBECONFORM) ## helm lint and helm template with every values combination, schema rejections, kubeconform
	@mkdir -p bin/kubeconform-cache
	HELM=$(abspath $(HELM)) KUBECONFORM=$(abspath $(KUBECONFORM)) \
		KUBECONFORM_FLAGS="-cache $(abspath bin/kubeconform-cache) -schema-location default -schema-location $(CRDS_CATALOG)" \
		go test -tags chart -count=1 ./$(CHART)/test/

chart-package: $(HELM) ## Package the chart into bin/charts; CHART_VERSION and APP_VERSION override Chart.yaml
	$(HELM) package $(CHART) --destination bin/charts \
		$(if $(CHART_VERSION),--version $(CHART_VERSION)) $(if $(APP_VERSION),--app-version $(APP_VERSION))

chart-push: $(HELM) ## Push the CHART_VERSION package from bin/charts to CHART_REGISTRY (after helm registry login)
	@test -n "$(CHART_VERSION)" || { echo "chart-push: set CHART_VERSION"; exit 1; }
	$(HELM) push bin/charts/wallet-accountant-$(CHART_VERSION).tgz $(CHART_REGISTRY)

print-helm: $(HELM)
	@echo $(abspath $(HELM))

$(HELM):
	GOBIN=$(abspath $(dir $(HELM))) go install helm.sh/helm/v4/cmd/helm@$(HELM_VERSION)

$(KUBECONFORM):
	GOBIN=$(abspath $(dir $(KUBECONFORM))) go install github.com/yannh/kubeconform/cmd/kubeconform@$(KUBECONFORM_VERSION)

lint-go:
	go tool golangci-lint run ./...

lint-api:
	@# A tree (the Category tree) recurses through an array, which always ends.
	$(VACUUM) --details --fail-severity warn --ignore-array-circle-ref --ignore-file api/vacuum-ignore.yaml api/openapi.yaml
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

# Renovate (ADR-0020) keeps every pinned version current in this repo and the
# env repo. Each list names, per manager, one dependency it must find, so a
# config change that stops a manager from matching fails here; check.sh also
# fails when one dependency's updates would land in several PRs.
# Updates the env repo's Renovate must put in one PR (deploy/renovate/check.sh;
# the env repo's README, "Renovate"): a chart with the images it renders, and
# a version with the images.lock entries it decides.
RENOVATE_ENV_TOGETHER := \
	together:fluxcd/flux2,ghcr.io/fluxcd/source-controller,ghcr.io/fluxcd/kustomize-controller,ghcr.io/fluxcd/helm-controller,ghcr.io/fluxcd/notification-controller \
	together:rancher/k3s,k3s-io/k3s \
	together:quay.io/jetstack/charts/cert-manager,quay.io/jetstack/cert-manager-controller,quay.io/jetstack/cert-manager-cainjector,quay.io/jetstack/cert-manager-webhook,quay.io/jetstack/cert-manager-startupapicheck \
	together:headlamp,ghcr.io/headlamp-k8s/headlamp \
	together:cloudnative-pg,ghcr.io/cloudnative-pg/cloudnative-pg \
	together:cert-manager-webhook-ovh,ghcr.io/aureq/cert-manager-webhook-ovh \
	together:prometheus,loki,tempo,grafana,quay.io/prometheus/prometheus,quay.io/prometheus-operator/prometheus-config-reloader,docker.io/grafana/loki,docker.io/grafana/tempo,docker.io/grafana/grafana
RENOVATE_EXPECT := \
	gomod:go gomod:github.com/shopspring/decimal \
	github-actions:actions/checkout github-actions:actions/setup-go \
	dockerfile:golang dockerfile:gcr.io/distroless/static-debian13 \
	helm-values:otel/opentelemetry-collector-contrib helm-values:curlimages/curl \
	regex:helm.sh/helm/v4 regex:github.com/yannh/kubeconform regex:golang.org/x/vuln \
	dockerfile:nginxinc/nginx-unprivileged regex:mcr.microsoft.com/playwright \
	npm:@playwright/test npm:oidc-client-ts \
	regex:k3d-io/k3d regex:FiloSottile/mkcert
RENOVATE_ENV_EXPECT := \
	regex:nginxinc/nginx-unprivileged \
	flux:quay.io/jetstack/charts/cert-manager flux:headlamp \
	kubernetes:ghcr.io/headlamp-k8s/headlamp-plugin-flux kubernetes:rancher/k3s kubernetes:swaggerapi/swagger-ui \
	regex:swaggerapi/swagger-ui \
	regex:quay.io/jetstack/charts/cert-manager regex:headlamp \
	regex:rancher/k3s regex:ghcr.io/headlamp-k8s/headlamp-plugin-flux \
	flux:prometheus flux:loki flux:tempo flux:grafana \
	regex:prometheus regex:loki regex:tempo regex:grafana \
	kubernetes:docker.io/curlimages/curl regex:docker.io/curlimages/curl \
	kubernetes:registry regex:registry regex:golang regex:quay.io/jetstack/cert-manager-controller \
	flux:cert-manager-webhook-ovh regex:cert-manager-webhook-ovh regex:ghcr.io/aureq/cert-manager-webhook-ovh \
	regex:mikefarah/yq regex:ghcr.io/yannh/kubeconform regex:binwiederhier/ntfy \
	regex:k3s-io/k3s regex:tailscale/tailscale regex:fluxcd/flux2 regex:Deuxfleurs/garage \
	terraform:flux terraform:http terraform:kubernetes terraform:onepassword terraform:ovh \
	terraform:proxmox terraform:random terraform:tailscale terraform:tls \
	flux:otel/opentelemetry-collector-contrib flux:curlimages/curl \
	$(RENOVATE_ENV_TOGETHER)

renovate-check: ## Validate renovate.json here and in ENV_REPO, and dry-run Renovate's lookups in Docker
	deploy/renovate/check.sh . $(RENOVATE_EXPECT)
	deploy/renovate/check.sh $(ENV_REPO) $(RENOVATE_ENV_EXPECT)
