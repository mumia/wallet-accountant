# 01: Serve the API under /api

**What to build:** every API path starts with `/api`, so the UI and the API can share one origin with plain path-prefix routing. `GET /api/me` behaves as `GET /me` does today, and `GET /me` is 404. Spec: `../spec.md` ("API"); ADR-0021.

**Blocked by:** None (can start immediately)

**Status:** claimed

- [ ] `api/openapi.yaml`: `servers` is `/api`; vacuum with the Zalando ruleset still passes
- [ ] The HTTP server mounts the ogen handler under `/api`; anything outside it is 404 problem+json
- [ ] `make generate` regenerates the server and the Bruno collection; the Bruno base URL includes `/api`
- [ ] Problem types stay relative URIs and still resolve (check `/problems/registration-required` against the new prefix and decide; record it in the conventions doc)
- [ ] HTTP tests call `/api/...`; one test proves an unprefixed path is 404
- [ ] README, `api/bruno/README.md` and the conventions doc show the new paths

## Comments

- Implemented: `httpapi.PathPrefix = "/api"` mounts the ogen server with `oas.WithPathPrefix`; ogen's own NotFound hook answers anything outside it as 404 problem+json. A unit test keeps the contract's `servers` (`/api`) and `PathPrefix` in step.
- The Bruno generator turns an origin-relative server URL into `http://localhost:8080/api` for `baseUrl`.
- Problem types: kept as `/problems/...` without the prefix (recorded in `docs/architecture/conventions.md`, "API paths"). They resolve against the request URI to `https://<host>/problems/...`, a UI-owned path; they are identifiers compared as sent, so no client-visible type changes.
- Also updated `deploy/selfhosted/smoke.sh` to call `/api/me` and `/api/registrations` (not run here; `bash -n` only). Swagger UI in `compose.dev.yaml` has submit disabled, so the relative server URL doesn't affect it.
