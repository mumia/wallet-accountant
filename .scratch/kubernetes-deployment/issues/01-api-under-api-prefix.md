# 01: Serve the API under /api

**What to build:** every API path starts with `/api`, so the UI and the API can share one origin with plain path-prefix routing. `GET /api/me` behaves as `GET /me` does today, and `GET /me` is 404. Spec: `../spec.md` ("API"); ADR-0021.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] `api/openapi.yaml`: `servers` is `/api`; vacuum with the Zalando ruleset still passes
- [ ] The HTTP server mounts the ogen handler under `/api`; anything outside it is 404 problem+json
- [ ] `make generate` regenerates the server and the Bruno collection; the Bruno base URL includes `/api`
- [ ] Problem types stay relative URIs and still resolve (check `/problems/registration-required` against the new prefix and decide; record it in the conventions doc)
- [ ] HTTP tests call `/api/...`; one test proves an unprefixed path is 404
- [ ] README, `api/bruno/README.md` and the conventions doc show the new paths
