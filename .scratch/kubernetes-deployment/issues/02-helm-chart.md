# 02: Helm chart for Wallet Accountant

**What to build:** `deploy/helm/wallet-accountant`, the app's installable package. `helm install` with a values file runs migrations, then the app in `all` mode, its collector and routes, against either a database it creates itself or one passed in, and against Zitadel (configured by the chart) or any OIDC provider. Spec: `../spec.md` ("The chart", user stories 17–19); ADR-0019, ADR-0020, ADR-0022, ADR-0023.

**Blocked by:** 01

**Status:** claimed

- [ ] Deployment running `all`, 1 replica, non-root, read-only root filesystem, liveness and readiness probes; values allow splitting into `api` and `worker` later
- [ ] `migrate` as a `pre-install`/`pre-upgrade` hook Job; only this Job gets the owner database credentials (`WA_OWNER_DATABASE_URL`)
- [ ] `database.mode: external` (Secret names for the owner, app and reader URLs) and `managed` (a single-tenant CNPG `Cluster` plus the owner role; the owner role creation replaces `postgres-init.sh`)
- [ ] An OpenTelemetry Collector Deployment; exporters from values, with a discard default (ADR-0016)
- [ ] Gateway API `HTTPRoute`: `/api` to the app; every other path to the UI when `ui.enabled`. Gateway reference, hostname and TLS from values
- [ ] UI slot, off by default: image, tag, `ui.configPath`, and the `config.json` ConfigMap built from values (`docs/architecture/ui-contract.md`)
- [ ] `oidc.zitadelSetup.enabled`: a `post-install`/`post-upgrade` hook Job porting `deploy/zitadel/setup-project.sh`. It is idempotent and creates or updates the project (ID = audience), `api-clients`, and `web` (User Agent, PKCE, refresh tokens, redirect and post-logout URIs from the UI host). It sets 15-minute access tokens and writes the generated client IDs to a ConfigMap the UI config and Make targets read. With setup off, the issuer, audience and client IDs come from values
- [ ] Pod templates for `bootstrap` and `rebuild` that Make targets can run as one-off pods (e.g. `kubectl run --overrides` rendered by `helm template --show-only`)
- [ ] `values.schema.json` so bad values fail at install
- [ ] CI: `helm lint` and `helm template` for every combination of database mode, Zitadel setup and UI slot
- [ ] Chart README: the values, the two self-hosting paths (homelab platform or standalone with `managed`)
