# 14: Remove Compose

**What to build:** Compose is gone from the repo, and the docs describe only the Kubernetes paths. Spec: `../spec.md` ("Removing Compose"); ADR-0019.

**Blocked by:** 06, 08, 11

**Status:** ready-for-agent

- [ ] Delete `compose.yaml`, `compose.dev.yaml`, `deploy/selfhosted/`, `deploy/dev/` and `deploy/zitadel/setup-project.sh` (ported in ticket 02)
- [ ] Remove the Compose Make targets (`up`, `up-observability`, `down`, `logs`, `bootstrap`, `compose-rebuild`, `smoke`, `dev-*`, `grafana`, `swagger`), or point them at their `local-*` replacements
- [ ] The integration tests' testcontainers PostgreSQL stays: it is a test dependency, not a way to run the app
- [ ] README and `docs/architecture/conventions.md` describe local, debug, smoke and production; the layout table lists `deploy/helm`
- [ ] Check that no doc or ADR still tells someone to run Compose (ADR-0017's example command updated to the Make target)
