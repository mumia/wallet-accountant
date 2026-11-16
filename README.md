# wallet-accountant
Your accountant in your wallet

## Development

Needs Go (see `go.mod`) and Docker. Every tool is pinned in `go.mod` and run with `go tool`.

```sh
make dev-up    # PostgreSQL, Zitadel (http://localhost:8081) and otel-lgtm (Grafana on http://localhost:3000)
make run       # the app on http://localhost:8080
make lint test test-integration
```

## Self-hosting

Needs Docker, and `openssl` for the first `make up`.

```sh
make up          # the app on http://localhost:8080, Zitadel on http://auth.localhost:8081
make bootstrap   # prints the one-time Invitation code for the first Instance administrator
```

The first `make up` writes random secrets to `.env`; keep that file. `make up-observability` also starts Tempo, Prometheus, Loki and Grafana (http://localhost:3000, user `admin`, password `GRAFANA_ADMIN_PASSWORD` in `.env`). In Zitadel, create a project for Wallet Accountant and set `OIDC_AUDIENCE` in `.env` to its ID, then run `make up` again; the other settings are listed at the end of `.env`. Sign in, then redeem the code with `POST /registrations`. `make smoke` checks the whole path in a throwaway Compose project.

`make help` lists every target. The conventions are in `docs/architecture/conventions.md`.
