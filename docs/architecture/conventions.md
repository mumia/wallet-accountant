# Engineering conventions

Decisions that every bounded context follows but that are not surprising enough to need an ADR. Architectural decisions are in `docs/adr/`.

## Layout

```
cmd/wallet-accountant/          main and run modes (ADR-0009)
api/openapi.yaml                the contract (ADR-0008)
api/zalando-ruleset.yaml        the JavaScript-free Zalando ruleset vacuum lints the contract with (ADR-0012)
internal/<context>/
    domain/                     Deciders, events, value objects; imports only the standard library and sharedkernel
    app/                        command and query handlers, port interfaces
    adapters/{http,postgres,…}/ ogen handlers, sqlc repositories, Watermill handlers
internal/sharedkernel/          Money, currencies, IDs, Match
    decider/                    the Decider contract aggregates implement; decidertest/ for given/when/then tests
internal/platform/              event store, Decider runtime, codec, Watermill wiring, config
    eventstore/                 event store, CloudEvents codec, Decider runtime, rebuild; eventstoretest/ for tests
    forwarding/                 Watermill SchemaAdapter, handler middleware, Drain for tests
    database/migrations/        golang-migrate migrations, run as wa_owner by the migrate run mode
    database/databasetest/      a fresh migrated testcontainers database per test, with the three role URLs; row-level security assertions
    httpapi/                    problem+json errors, bearer authentication, Idempotency-Key, request telemetry
    httpapi/oas/                the ogen server generated from api/openapi.yaml (`make generate`)
    authn/                      the Authenticator port; oidc/ verifies JWTs, authntest/ is for tests
    telemetry/                  slog, traces and metrics (ADR-0016)
internal/wiring/                composition root: builds every context and runs a run mode
internal/apitest/               HTTP test harness: the real app in-process against testcontainers PostgreSQL
```

depguard rules in golangci-lint enforce the dependencies. `domain` imports nothing from `app`, `adapters` or `platform`. A context uses another context only through that context's `app` ports.

## Identifiers

All aggregate and event IDs are UUIDv7. The application generates them, never the database, so a retried command produces the same IDs.

## Idempotency

- POST endpoints accept an `Idempotency-Key` header, following the Zalando guidelines. An HTTP middleware in `platform/httpapi` honours it on every POST, so a new endpoint only declares the `IdempotencyKey` parameter in `api/openapi.yaml` (a unit test checks every POST does). Keys belong to the caller's OIDC identity and are reserved before the request runs, so a concurrent retry gets 409 instead of running twice. A 2xx response is stored and replayed for the same key and body; the same key with another body gets 422; any other response releases the key. A response holding a Household's data is stored in that Household's scope: the Household in the path, or the one a handler names with `httpapi.ResponseBelongsTo`.
- A Statement import is deduplicated by a hash of the file. An Imported transaction is deduplicated by the bank's own transaction ID when the bank provides one.
- Watermill delivers events at least once, and commits the handler's transaction even when a message is not acked (ADR-0011). Every event handler wraps each attempt in a `SAVEPOINT`, rolls back to it before a Nack, and records `(consumer_group, event_id)` in a processed-events table in the same transaction, skipping events it has already recorded.

## Testing

- Deciders: pure given/when/then tests. Given these past events, when this command, then these new events or this error.
- Anything that touches PostgreSQL: integration tests against a real database through testcontainers-go.
- HTTP: contract tests generated from `api/openapi.yaml`.
- Household isolation: tests confirm that both the query filters and the row-level security policies (ADR-0014) block access to another Household's data. HTTP tests show another Household's resources answer 404. Every migration that adds a Household-scoped table adds a `databasetest.AssertHouseholdIsolation` test for it, and every instance-level table an `AssertInstanceScopeOnly` test; both bypass the query filters.
- Household-scoped requests run through Households' `InHousehold`: one transaction as `wa_app` that resolves the caller's User in instance scope, sets `app.household_id` from the path, and answers 404 to non-members and unregistered callers.

## Configuration and observability

- Configuration comes only from environment variables: `WA_DATABASE_URL` (as `wa_app`), `WA_OIDC_ISSUER` and `WA_OIDC_AUDIENCE` (required), `WA_READER_DATABASE_URL` (as `wa_event_store_reader`; needed by `worker`, `all` and `rebuild`), `WA_OWNER_DATABASE_URL` (as `wa_owner`; needed by `migrate`, which also sets the passwords of `wa_app` and `wa_event_store_reader` to those in their URLs), `WA_HTTP_ADDR` (default `:8080`), `WA_SHUTDOWN_TIMEOUT` (default `10s`), and the standard `OTEL_*` variables. The run mode is the binary's only argument (default `all`).
- Logs use `log/slog` with JSON output.
- Logs, traces and metrics are exported over OTLP by default. Logs also go to stdout as JSON; slog reaches OTLP through the OpenTelemetry slog bridge.
- Telemetry goes to an OpenTelemetry Collector, which forwards it to the Grafana stack: Tempo for traces, Prometheus for metrics, Loki for logs, Grafana to view them (ADR-0016). Development uses the all-in-one `otel-lgtm` image from `compose.dev.yaml`; self-hosting runs the collector and enables the backends with the `observability` profile (`make up-observability`).
- Events carry W3C trace context (`traceparent`, ADR-0015), so a Watermill handler continues the trace of the request that caused the event.

## Make targets

Every way of running the project has a Make target. Go tools are pinned in `go.mod` and run with `go tool`, so nothing has to be installed globally.

| Target | What it does |
|---|---|
| **Development** (`compose.dev.yaml`; the app runs on the host) | |
| `make dev-up` / `make dev-down` | Start or stop PostgreSQL, Zitadel and `otel-lgtm` |
| `make run` | Run the app in `all` mode against the dev dependencies |
| `make run-api` / `make run-worker` | Run a single run mode |
| `make migrate` | Run the `migrate` run mode against the dev database |
| `make rebuild` | Run the `rebuild` run mode against the dev database |
| `make dev-bootstrap` | Run the `bootstrap` run mode against the dev database |
| `make grafana` | Open the local Grafana UI |
| **Self-hosted** (`compose.yaml`; everything in Docker) | |
| `make up` / `make down` | Start or stop the app, migrations, PostgreSQL, Zitadel and the collector |
| `make up-observability` | Same as `make up`, plus Tempo, Prometheus, Loki and Grafana |
| `make logs` | Follow the app's logs |
| `make bootstrap` | Run the `bootstrap` run mode once and print the first Instance administrator Invitation code (ADR-0017) |
| **Build and quality** | |
| `make generate` | Regenerate code from `api/openapi.yaml` (ogen) and SQL queries (sqlc) |
| `make lint` | Run golangci-lint (including depguard) and vacuum with the Zalando ruleset |
| `make test` | Run the Decider and other unit tests |
| `make test-integration` | Run the integration tests against PostgreSQL through testcontainers-go |
| `make build` / `make docker-build` | Build the binary or the container image |
