# 02: Helm chart for Wallet Accountant

**What to build:** `deploy/helm/wallet-accountant`, the app's installable package. `helm install` with a values file runs migrations, then the app in `all` mode, its collector and routes, against either a database it creates itself or one passed in, and against Zitadel (configured by the chart) or any OIDC provider. Spec: `../spec.md` ("The chart", user stories 17–19); ADR-0019, ADR-0020, ADR-0022, ADR-0023.

**Blocked by:** 01

**Status:** resolved

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

## Comments

- Implemented in `deploy/helm/wallet-accountant`. Checks are Go tests behind the `chart` build tag in `test/` (helm lint `--strict` and helm template for all 8 combinations, per-feature assertions on the rendered objects, 23 schema/template rejection cases, kubeconform with the CNPG and Gateway API schemas from the CRDs catalog at a pinned commit). `make lint-chart` builds helm v4.3.0 and kubeconform v0.7.0 into `bin/` with `go install` and runs them; `make lint` calls it, so CI's `lint` job covers it. kubeconform fetches schemas over the network (cached in `bin/kubeconform-cache`).
- Probes: the app has no health endpoint, so liveness and readiness are TCP checks on the HTTP port. A `/healthz` in the app would be a small follow-up.
- Managed database: the CNPG `Cluster` and its two Secrets (owner basic-auth, generated URLs) are `pre-install` hooks with `hook-delete-policy: hook-failed`, because they must exist before the `migrate` pre-install hook; that policy stops Helm's default before-hook-creation from deleting them. Consequence, documented in the README: Helm creates them once, never updates them on upgrade and never deletes them on uninstall. `wa_owner` is the initdb owner plus a CNPG managed role with `login`, `createrole`, `bypassrls`. migrate's `backoffLimit: 10` covers the cluster starting; the README asks for `--timeout 15m` on the first install.
- Zitadel setup (`files/zitadel-setup.sh`, curl image): the Job, not the chart, owns the `<fullname>-oidc` ConfigMap when setup is on (server-side apply via the in-cluster API), so Helm never reverts the generated client IDs. With setup off, the chart renders the same ConfigMap from values, so the UI and Make targets read one place either way. The UI mounts `config.json` from it (`optional: true` with setup on, since it only exists after the first post-install run) and the Job restarts the UI Deployment by patching a `checksum/config` annotation when `config.json` changes. RBAC: create configmaps, get/patch only that ConfigMap and only the UI Deployment.
- Not verified against a live Zitadel (only `sh -n`): the v2 `UpdateApplication` body shape, `developmentMode`, the `applicationId` field name in `ListApplications`, and `PUT /admin/v1/settings/oidc` for lifetimes (written only when they differ; a "not changed" answer is tolerated). Ticket 05/08's smoke test and stand-in UI will exercise them.
- `trust.caConfigMap` (extra CA, e.g. local mkcert) is mounted for the app/migrate/operator pods via `SSL_CERT_DIR` and for the setup Job via `CURL_CA_BUNDLE`; ticket 05 will need it for `auth.localhost`.
- Operator pods: `--set operatorPod.mode=bootstrap|rebuild --show-only templates/operator-pod.yaml` prints a Pod as JSON for `kubectl run wallet-accountant-<mode> --overrides` (the README has the command). The Make targets themselves are left to ticket 05, which owns the `local-*` targets.
- `collector.enabled: false` without `collector.externalEndpoint` sets `OTEL_{TRACES,METRICS,LOGS}_EXPORTER=none`.
