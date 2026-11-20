# wallet-accountant
Your accountant in your wallet

## Development

Needs Go (see `go.mod`), Docker, k3d, kubectl, flux, mkcert and jq, and a checkout of the env repo `wallet-accountant-deploy` next to this one (or `ENV_REPO=<path>`). Every Go tool is pinned in `go.mod` and run with `go tool`. Development runs on a local Kubernetes cluster that mirrors production (ADR-0019); the env repo's README describes what runs in it.

```sh
make local-ca       # once per Mac: create the mkcert CA and trust it (asks for your password)
make local-prefetch # online, once: mirror the env repo's images.lock, so the rest works offline
make local-up       # the k3d cluster: Traefik, cert-manager, CloudNativePG (apps-db), Zitadel, Headlamp, Swagger UI
make local-deploy   # build the image and chart from this working tree and roll them out through Flux
make local-bootstrap   # print a bootstrap Invitation code (only to this terminal)
make lint test test-integration
```

Every URL carries the cluster's port, 18443 (`LOCAL_HTTPS_PORT`), so another cluster can keep 80 and 443:

| What | Where |
|---|---|
| The API | `https://wallet.localhost:18443/api` |
| Zitadel (the issuer) | `https://auth.localhost:18443`, console `/ui/console`; login and password in the env repo's README |
| Swagger UI, showing `api/openapi.yaml` (read-only) | `https://swagger.localhost:18443` |
| Headlamp | `https://headlamp.localhost:18443` (`make local-headlamp-token`) |

`make local-deploy` gives every build a unique image tag and chart version, pushes both to the cluster's registry (`localhost:5050`), records them in `bin/local-build.env` and in the env repo's artifact, which it pushes with them, and waits for Flux to roll them out. Migrations run as the chart's hook before the app starts, and the chart's Zitadel setup Job creates the project `wallet-accountant` (the audience) with the applications `api-clients` and `web`. It then writes the `api-clients` client ID to `api/bruno/.env`. `make local-rebuild` rebuilds the folds in the cluster. `make local-check` checks the whole local environment, the app included; `make local-down` deletes the cluster (the registry's storage is kept, and the next `make local-up` deploys the last build again).

**Bruno.** Open `api/bruno` in Bruno 3 and choose the environment `local` for the app in the cluster; with no environment, the collection calls the app from `make run`. Both sign in to the cluster's Zitadel (see `api/bruno/README.md`).

**Debug mode.** `make run` (and `run-api`, `run-worker`, `migrate`, `dev-bootstrap`) runs the app on this Mac, under `http://localhost:8080/api`, against the cluster's apps-db on `localhost:5432` (`LOCAL_DB_PORT`), its collector on `localhost:4318` (`LOCAL_OTLP_PORT`) and `WA_OIDC_ISSUER=https://auth.localhost:18443`. Its defaults are in the `Makefile`; an IDE's debugger needs the same variables. Go trusts the mkcert CA through the macOS keychain, so nothing else is needed. The collector exists once the app is deployed (`make local-deploy`).

CI (`.github/workflows/ci.yaml`) runs the same lint, generate, tidy, test, integration test, image build and govulncheck checks on every pull request; the list is in `docs/architecture/conventions.md`.

`make dev-up` still starts the former Compose dependencies (PostgreSQL, Zitadel on `http://localhost:8081`, Swagger UI, otel-lgtm) until they are removed; the run targets no longer point at them, and its PostgreSQL needs `localhost:5432`, which the cluster holds.

## Production

Production is the env repo's `clusters/prod`: a k3s VM on the homelab, at `https://wallet.shire.migmar.eu`, reachable only on the tailnet. Releases reach it by themselves: Flux image automation follows this repo's semver tags (`docs/architecture/conventions.md`, "Releases"). With the admin kubeconfig at `~/.kube/shire.yaml` (`PROD_KUBECONFIG`; the env repo's `infra/README.md`):

```sh
make prod-check       # build and check clusters/prod without a cluster (REQUIRE_ENCRYPTED=1: every Secret sealed)
make prod-bootstrap   # print production's bootstrap Invitation code (only to this terminal)
make prod-bruno-env   # production's api-clients client ID for Bruno's environment prod
```

## Self-hosting

Needs Docker, and `openssl` for the first `make up`.

```sh
make up          # the app, with the API under http://localhost:8080/api, Zitadel on http://auth.localhost:8081
make bootstrap   # prints the one-time Invitation code for the first Instance administrator
```

The first `make up` writes random secrets to `.env`; keep that file. `make up-observability` also starts Tempo, Prometheus, Loki and Grafana (http://localhost:3000, user `admin`, password `GRAFANA_ADMIN_PASSWORD` in `.env`). `make up` also creates the Zitadel project Wallet Accountant's users sign in to, with the ID in `OIDC_AUDIENCE` (default `wallet-accountant`), so the audience needs no manual step; the other settings are listed at the end of `.env`. An installation whose Zitadel was first started before this setup existed has no setup token: create the project in the console and set `OIDC_AUDIENCE` to its ID. Sign in, then redeem the code with `POST /api/registrations`.

`make help` lists every target. The conventions are in `docs/architecture/conventions.md`.
