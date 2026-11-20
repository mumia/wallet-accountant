# 14: Remove Compose

**What to build:** Compose is gone from the repo, and the docs describe only the Kubernetes paths. Spec: `../spec.md` ("Removing Compose"); ADR-0019.

**Blocked by:** 06, 08, 11

**Status:** claimed

- [ ] Delete `compose.yaml`, `compose.dev.yaml`, `deploy/selfhosted/`, `deploy/dev/` and `deploy/zitadel/setup-project.sh` (ported in ticket 02)
- [ ] Remove the Compose Make targets (`up`, `up-observability`, `down`, `logs`, `bootstrap`, `compose-rebuild`, `smoke`, `dev-*`, `grafana`, `swagger`), or point them at their `local-*` replacements
- [ ] The integration tests' testcontainers PostgreSQL stays: it is a test dependency, not a way to run the app
- [ ] README and `docs/architecture/conventions.md` describe local, debug, smoke and production; the layout table lists `deploy/helm`
- [ ] Check that no doc or ADR still tells someone to run Compose (ADR-0017's example command updated to the Make target)

## Comments

- **Removed.** `compose.yaml`, `compose.dev.yaml`, `deploy/selfhosted/`, `deploy/dev/`, `deploy/zitadel/setup-project.sh`, and the Make targets `up`, `up-observability`, `down`, `logs`, `bootstrap`, `compose-rebuild`, `dev-up`, `dev-down`, `dev-reset`, `grafana`, `swagger` with the `COMPOSE*` and `ENV_FILE` variables (`smoke` was already gone, ticket 08). None was repointed at a `local-*` target: each has a differently named replacement (`local-up`, `local-up-observability`, `local-bootstrap`, `local-rebuild`), and the old URLs (`localhost:3000`, `:8082`) no longer exist; Grafana and Swagger UI are now rows in the README's URL table. Nothing in this repo or the env repo read `deploy/selfhosted` (ticket 07 copied the Grafana data sources).
- **Kept.** The testcontainers PostgreSQL in `databasetest` and `apitest` (`make test-integration`), and `.gitignore`'s `.env` (it still covers `api/bruno/.env`). Comments that named the deleted files now point at the CloudNativePG managed role (migration 000001, `databasetest`), the chart (`Dockerfile`), or drop the reference (the chart's `database-managed.yaml`, `files/zitadel-setup.sh` and README). Migration 000001 changed in a comment only.
- **Docs.** README: the `dev-up` paragraph and the Compose self-hosting section are gone; a smoke-test paragraph, a Grafana row and a short self-hosting section (install the chart on k3s, see the chart's README) are added. `conventions.md`: the layout lists `Dockerfile` and `deploy/helm` (no Compose rows); the telemetry bullet describes the chart's collector and `local-up-observability`; the targets table loses the Compose rows and gains `local-up-observability` and a Production group (`prod-check`, `prod-bootstrap`, `prod-bruno-env`).
- **ADRs.** Only ADR-0017's example changed (`make local-bootstrap` / `make prod-bootstrap`). ADR-0006, 0009, 0010, 0016, 0019, 0022 and 0023 still name Compose as history; each is already marked superseded or amended by ADR-0019/0022 and none tells anyone to run it.
- **Test first.** A throwaway acceptance script (not committed: a test for absence would only guard deleted files) failed on 58630cb with 81 findings (files, targets, `docker compose` in the Makefile, doc instructions including ADR-0017's command) and passes now.
- **Checks.** `make lint` (with `lint-chart`, `lint-workflows`), `make test`, `make test-integration`, `make renovate-check` (no expected dependency came from the Compose files, so `RENOVATE_*EXPECT` and `renovate.json` are unchanged), `make images-lock-check` (`LOCK_DOCKERFILES` unchanged) and `make prod-check` pass. Live: `make local-down && make local-up && make local-deploy && make local-check` passed (109 checks). A first attempt failed when Docker restarted mid-deploy; a killed retry left its `wa-registry-prefetch` container holding port 5050, which `local-up` then could not bind (removed by hand).
