# wallet-accountant
Your accountant in your wallet

## Development

Needs Go (see `go.mod`) and Docker. Every tool is pinned in `go.mod` and run with `go tool`.

```sh
make dev-up    # PostgreSQL, Zitadel (http://localhost:8081), Swagger UI (http://localhost:8082) and otel-lgtm (Grafana on http://localhost:3000)
make migrate   # apply the database migrations
make run       # the app on http://localhost:8080
make lint test test-integration
```

Swagger UI shows the contract, `api/openapi.yaml`. To send requests, open `api/bruno` in Bruno 3: it signs in to Zitadel by itself (see `api/bruno/README.md`).

CI (`.github/workflows/ci.yaml`) runs the same lint, generate, tidy, test, integration test, image build and govulncheck checks on every pull request; the list is in `docs/architecture/conventions.md`.

`make dev-up` also creates the Zitadel project `wallet-accountant`, the audience the app expects, and in it the public application `api-clients` that the Bruno collection signs in with. When a change alters the database roles or the migrations, or you just want a clean slate, `make dev-reset` deletes the dev database and Zitadel data, starts the dependencies again and migrates.

## Self-hosting

Needs Docker, and `openssl` for the first `make up`.

```sh
make up          # the app on http://localhost:8080, Zitadel on http://auth.localhost:8081
make bootstrap   # prints the one-time Invitation code for the first Instance administrator
```

The first `make up` writes random secrets to `.env`; keep that file. `make up-observability` also starts Tempo, Prometheus, Loki and Grafana (http://localhost:3000, user `admin`, password `GRAFANA_ADMIN_PASSWORD` in `.env`). `make up` also creates the Zitadel project Wallet Accountant's users sign in to, with the ID in `OIDC_AUDIENCE` (default `wallet-accountant`), so the audience needs no manual step; the other settings are listed at the end of `.env`. An installation whose Zitadel was first started before this setup existed has no setup token: create the project in the console and set `OIDC_AUDIENCE` to its ID. Sign in, then redeem the code with `POST /registrations`. `make smoke` checks the whole path in a throwaway Compose project.

`make help` lists every target. The conventions are in `docs/architecture/conventions.md`.
