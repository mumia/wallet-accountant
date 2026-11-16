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

## Comments

Implementation notes (implementer agent):

- The spike branch `prototype/spike-eventstore-ogen` is empty; the spike was read from dangling commit `569e9e5`. The JavaScript-free Zalando ruleset was never committed there; the recovered copy is now `api/zalando-ruleset.yaml`, and its deliberately broken probe spec is `api/testdata/zalando-violations.yaml`. `make lint` fails if the ruleset stops reporting violations in that probe. Consider tagging `569e9e5` before a `git gc` removes it.
- Generated ogen code lives in `internal/platform/httpapi/oas` (server only, with response validation, so HTTP tests also check responses against the contract). The composition root is `internal/wiring`; the HTTP harness is `internal/apitest`. The conventions doc's layout lists them.
- The Authenticator port is `internal/platform/authn` (identity = issuer + subject); `authn/oidc` verifies JWTs through discovery and JWKS (lazily, so the API starts before Zitadel is up); `authn/authntest` maps test tokens to identities.
- The `registration-required` problem type is the relative URI `/problems/registration-required`; generic errors use `about:blank`.
- `GET /me` always answers 403 for now: there is no User store until ticket 03.
- `migrate`, `rebuild` and `bootstrap` are recognised run modes that exit with "not available yet"; `worker` only waits for shutdown. Tickets 02 and 03 fill them in. The harness and `make run` connect to PostgreSQL as a single dev user; the `wa_owner`, `wa_app` and `wa_event_store_reader` roles arrive with ticket 02's first migration, which should switch `deploy/dev/postgres-init.sql` and the harness to them.
- The "telemetry backend undecided" line was already gone from the conventions doc (the ADR-0016 text was in place); the doc now also lists the environment variables and the new packages.
- Tests outside the agreed HTTP seam: a unit test of the OIDC adapter against a local issuer, unit tests of the problem+json error hooks (decode details are cleaned of operation names and decode paths, which the current contract can't trigger over HTTP), and config parsing.
- `make dev-up` and `make run` were checked by hand: `GET /me` answers 401 problem+json, and the OIDC adapter discovered the dev Zitadel.
