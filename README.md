# wallet-accountant
Your accountant in your wallet

## Development

Needs Go (see `go.mod`) and Docker. Every tool is pinned in `go.mod` and run with `go tool`.

```sh
make dev-up    # PostgreSQL, Zitadel (http://localhost:8081) and otel-lgtm (Grafana on http://localhost:3000)
make run       # the app on http://localhost:8080
make lint test test-integration
```

`make help` lists every target. The conventions are in `docs/architecture/conventions.md`.
