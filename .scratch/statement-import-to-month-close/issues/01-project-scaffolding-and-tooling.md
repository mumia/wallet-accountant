# 01: Project scaffolding and tooling

**What to build:** the walking skeleton's outer shell. A developer can clone the repo, run `make dev-up` and `make run`, and call the API. `GET /me` answers 401 without a token, and 403 `application/problem+json` with type `registration-required` for an authenticated identity, because no User can exist yet. Everything later slices rely on for building, linting and testing is in place. Spec: `../spec.md` ("Walking skeleton", stories 65–72); ADR-0008, ADR-0009, ADR-0010, ADR-0012, ADR-0016.

**Blocked by:** None (can start immediately)

**Status:** claimed

- [ ] Go module with the layout from the conventions doc (`cmd`, `api`, `internal/<context>/{domain,app,adapters}`, `sharedkernel`, `platform`); Go tools pinned in `go.mod` and run with `go tool`
- [ ] One binary whose run mode is chosen at start-up (ADR-0009); `api` serves HTTP. Configuration comes only from environment variables
- [ ] `api/openapi.yaml` is contract-first; `make generate` generates the ogen server; the JavaScript-free Zalando ruleset from the spike lives in the repo and `make lint` runs vacuum on the contract
- [ ] `make lint` runs golangci-lint with depguard: `domain` imports only the standard library and sharedkernel, and contexts talk to each other only through `app` ports
- [ ] Every error is `application/problem+json` with a status always set, and decode details are cleaned of internal paths
- [ ] An `Authenticator` port with an OIDC/JWKS adapter that returns issuer and subject, and a test implementation that maps a test token to an identity
- [ ] slog JSON goes to stdout and to the OTLP bridge; OpenTelemetry traces and metrics go to the collector (ADR-0016); every request produces a log record, a trace and metrics
- [ ] `compose.dev.yaml` with PostgreSQL, Zitadel and `otel-lgtm`; `make dev-up`, `make dev-down`, `make run`, `make run-api`, `make grafana`, `make build`, `make test`, `make test-integration`
- [ ] HTTP test harness: the real app runs in-process against PostgreSQL from testcontainers-go, with the test Authenticator swapped in
- [ ] HTTP tests: `GET /me` returns 401 without a token and 403 `registration-required` with a valid test token
- [ ] The conventions doc no longer says the telemetry backend is undecided
