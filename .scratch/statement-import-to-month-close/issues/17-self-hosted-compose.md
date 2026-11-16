# 17: Self-hosted Compose

**What to build:** a self-hoster runs `make up` and gets the app, migrations, PostgreSQL, Zitadel and the collector, then `make bootstrap` to print the first Invitation code. `make up-observability` also starts Tempo, Prometheus, Loki and Grafana. Spec: `../spec.md` stories 65–66b, 71; ADR-0005, ADR-0016, ADR-0017.

**Blocked by:** 03 (Bootstrap and registration)

**Status:** claimed

- [ ] `compose.yaml` runs migrations in the `migrate` run mode before the app starts
- [ ] `make up`, `make down`, `make up-observability` (the `observability` profile), `make logs`, `make bootstrap`, `make docker-build`
- [ ] `make bootstrap` prints a one-time Invitation code and nothing about it reaches the logs
- [ ] `make rebuild` rebuilds every fold registered so far
- [ ] A smoke check: from a clean checkout, `make up` then `make bootstrap` gives a code that can be redeemed

## Comments

Implementation notes (implementer agent):

- **Files.** `compose.yaml`, `Dockerfile` (static binary on `distroless/static-debian13:nonroot`), `.dockerignore`, and `deploy/selfhosted/`: `init-env.sh`, `postgres-init.sh` (creates `wa_owner` and the database, like `deploy/dev/postgres-init.sql` but with the password from the env file), `otelcol/{discard,observability}.yaml`, `tempo.yaml`, Grafana datasource provisioning, `smoke.sh` and `compose.smoke.yaml`.
- **Secrets.** The first `make up` writes random secrets to `.env` (gitignored); compose.yaml refuses to start without them. `ENV_FILE` picks another file. Compose variables are deliberately not named `WA_*`, because the Makefile exports the dev `WA_*` values and the shell environment would override `.env`.
- **Issuer.** The issuer URL must be the same for the browser and for the app inside Compose. Zitadel listens on `AUTH_PORT` (default 8081) inside and outside, and has `AUTH_DOMAIN` (default `auth.localhost`) as a network alias, so `http://auth.localhost:8081` works from the app's container, and from the host in browsers and curl, which resolve `*.localhost` to loopback. A deployment behind a TLS reverse proxy would need more than this.
- **Audience.** Zitadel has no project for the app out of the box, so `OIDC_AUDIENCE` (default `wallet-accountant`) must be set to the ID of a project the operator creates; README says so. Automating that (e.g. Zitadel's first-instance setup or a setup step) could be a later ticket.
- **Collector.** Without the profile it uses `discard.yaml` (`nop` exporter). `make up-observability` sets `COLLECTOR_CONFIG=observability`, which forwards over OTLP to Tempo, Prometheus (`--web.enable-otlp-receiver`) and Loki. `make down` stops the profile's services too.
- **Targets.** `up`, `up-observability`, `down`, `logs`, `bootstrap` (`docker compose run --rm --no-deps app bootstrap`, so the container and its output are removed), `docker-build`, plus `compose-rebuild` (the `rebuild` run mode against the self-hosted database; `make rebuild` stays the dev target) and `smoke`. conventions.md and README list them.
- **Smoke check.** `make smoke` runs the whole path as Compose project `wallet-accountant-smoke` on ports 18080/18081/13000 with fresh secrets, then removes it with its volumes: `make up`, `make bootstrap`, the code is in no container log, a Zitadel project and a machine identity with a JWT access token are created through the API (a PAT for the first-instance machine user comes from `compose.smoke.yaml`), the app is restarted with the project as audience, `POST /registrations` answers 201 and `GET /me` 200 (Instance administrator), a second bootstrap is refused, and `make compose-rebuild` keeps the registration. `SMOKE_UP=up-observability make smoke` also checks via Grafana that Tempo, Prometheus and Loki hold the app's data and that Loki does not hold the code (checked to fail when searching for a logged string instead). Both variants pass.
