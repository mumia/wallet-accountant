# 05: Wallet Accountant on the local cluster

**What to build:** `make local-up && make local-deploy` gives a working app at `https://wallet.localhost/api`. The developer can bootstrap and redeem an Invitation, call the API from Bruno, and switch to debug mode (`make run` on the host against the in-cluster dependencies). Spec: `../spec.md` (user stories 10, 13–15); ADR-0019.

**Blocked by:** 02, 04

**Status:** claimed

- [ ] `apps/wallet-accountant` HelmRelease with `database.mode: external` on `apps-db`, Zitadel setup on, UI off, at `wallet.localhost`
- [ ] `local-deploy` builds the image from this repo's working tree, pushes it to the k3d registry, and sets its tag in the pushed artifact (a unique tag per build, so Flux rolls it out)
- [ ] `make local-bootstrap` and `make local-rebuild` run the one-off pods from ticket 02. The code prints only to the terminal, and the pod is removed
- [ ] Debug mode: `apps-db` on `localhost:5432` and the collector on `localhost:4318`. `make run` defaults point at them, with `WA_OIDC_ISSUER=https://auth.localhost`. Go trusts the mkcert CA through the macOS keychain; verify it
- [ ] `api/bruno/.env` gets the `api-clients` client ID from the setup Job's ConfigMap; the Bruno environment's URLs are `https://wallet.localhost/api` and `https://auth.localhost`
- [ ] Swagger UI at `https://swagger.localhost` in the local cluster only, showing `api/openapi.yaml`
- [ ] README: the new development section
