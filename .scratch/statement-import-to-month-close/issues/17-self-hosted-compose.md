# 17: Self-hosted Compose

**What to build:** a self-hoster runs `make up` and gets the app, migrations, PostgreSQL, Zitadel and the collector, then `make bootstrap` to print the first Invitation code. `make up-observability` also starts Tempo, Prometheus, Loki and Grafana. Spec: `../spec.md` stories 65–66b, 71; ADR-0005, ADR-0016, ADR-0017.

**Blocked by:** 03 (Bootstrap and registration)

**Status:** claimed

- [ ] `compose.yaml` runs migrations in the `migrate` run mode before the app starts
- [ ] `make up`, `make down`, `make up-observability` (the `observability` profile), `make logs`, `make bootstrap`, `make docker-build`
- [ ] `make bootstrap` prints a one-time Invitation code and nothing about it reaches the logs
- [ ] `make rebuild` rebuilds every fold registered so far
- [ ] A smoke check: from a clean checkout, `make up` then `make bootstrap` gives a code that can be redeemed
