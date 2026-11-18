# Spec: Kubernetes deployment with a local environment that mirrors production

Status: ready-for-agent

Wallet Accountant moves from Docker Compose to Kubernetes. Production is a single k3s VM on the homelab's Proxmox server, reachable only over Tailscale with valid HTTPS. Locally, a k3d cluster runs the same platform and app definitions, fully offline, with the UI slot ready for when the UI exists. Decided in a grilling session on 2026-11-18. ADR-0019 to ADR-0023 record the decisions; `docs/architecture/ui-contract.md` is the UI's side.

## Problem Statement

Today the app is self-hosted with Compose, and development runs a different Compose file with the app on the host. Neither looks like the Kubernetes cluster the operator wants to run in production, so problems in routing, TLS, identity and delivery show up only when deploying. The UI is about to be built, and nothing yet says how it is served next to the API. The homelab has no infrastructure as code: rebuilding it after a disk failure would be done from memory.

## Solution

Three layers, each declared in git:

1. **Infra** (OpenTofu): the k3s VM and the Garage backup store on Proxmox, the Tailscale key, the DNS record and the Flux bootstrap.
2. **Platform** (Flux): cert-manager, Traefik with Gateway API, CloudNativePG with `zitadel-db` and `apps-db`, Zitadel, the Grafana stack, Headlamp, alerts.
3. **Apps** (Flux plus this repo's Helm chart): Wallet Accountant.

Layers 1 and 2 are homelab-wide and are written so they can move to their own repo later. The local environment is layers 2 and 3 on k3d, with local values.

## User Stories

### Operator: production

1. As the operator, I want one `tofu apply` to take the existing Proxmox host to a running k3s VM with Flux reconciling the env repo, so that rebuilding the homelab is a command, not a memory exercise.
2. As the operator, I want the app at `https://wallet.shire.migmar.eu` with a valid certificate, reachable only on my tailnet, so that it is private and works on every device without installing a CA.
3. As the operator, I want production to deploy only semver-tagged releases, bumped by Flux image automation in the env repo, so that shipping is deliberate and recorded in git.
4. As the operator, I want to follow deployments in Headlamp and be alerted on ntfy when a reconciliation fails, so that I know when a release didn't land.
5. As the operator, I want to sign in to Grafana and Headlamp with my Zitadel account, with a documented break-glass path, so that I have one login without being locked out when Zitadel is down.
6. As the operator, I want production secrets encrypted with SOPS in the env repo, and the keys that can't be regenerated kept in 1Password, so that the repo is the full source of truth and nothing critical lives only on the node.
7. As the operator, I want continuous PostgreSQL backups to Garage, plus nightly per-database dumps and a tested restore, so that I can recover from both disk loss and a bad migration.
8. As the operator, I want to issue the bootstrap Invitation with one command that prints the code only to my terminal, so that ADR-0017 still holds on Kubernetes.
9. As the operator, I want Renovate to propose version bumps for every pinned chart, image and tool, so that the stack doesn't rot.

### Developer: local

10. As a developer, I want `make local-up` to start a k3d cluster with the same platform and app as production, so that what works locally works in production.
11. As a developer, I want the local cluster to start and run with no network after a one-time `make local-prefetch`, so that I can work offline.
12. As a developer, I want local HTTPS on `wallet.localhost` and `auth.localhost` with a CA my Mac trusts, so that sign-in behaves exactly as in production.
13. As a developer, I want `make local-deploy` to ship my uncommitted changes through the local Flux, so that the delivery path is exercised on every check.
14. As a developer, I want to run the app on the host with a debugger against the in-cluster PostgreSQL, Zitadel and collector, so that I keep a fast debug loop.
15. As a developer, I want Bruno to keep working against the local cluster, with its client ID written to `api/bruno/.env`, so that I can call the API by hand.
16. As a developer, I want the Grafana stack locally behind `make local-up-observability`, so that I see telemetry when I need it without paying for it on every boot.

### Self-hoster (not the operator)

17. As a self-hoster, I want to install the app with `helm install` and a values file, choosing between a managed CNPG database and an external one, and between the bundled Zitadel setup and my own OIDC provider, so that I don't need this homelab's platform.

### UI

18. As the UI's developer, I want a written contract for the UI image (port, health, SPA fallback, runtime `config.json`, which paths it owns), so that the UI can be built and dropped into the chart's UI slot without backend changes.
19. As the UI's developer, I want the Zitadel `web` app and its redirect URIs created by the chart, so that sign-in works the moment the UI is deployed.

## Implementation Decisions

### Repos

- **`wallet-accountant`** (this repo): the app, `deploy/helm/wallet-accountant`, the UI contract, the CI that publishes image and chart, and the `local-*` Make targets. Those targets drive `../wallet-accountant-deploy` and build the image from this repo's source.
- **`wallet-accountant-deploy`** (new, **private**, on GitHub next to the other two):

  ```
  infra/
    bootstrap/      # OpenTofu: Garage LXC only; local state, encrypted, kept in 1Password
    homelab/        # OpenTofu: k3s VM, Tailscale key, OVH DNS, Flux bootstrap; state in Garage
  platform/         # Flux Kustomizations + HelmReleases; no reference to any app
  apps/
    wallet-accountant/
  clusters/
    prod/           # Flux entry point; values: *.shire.migmar.eu, Let's Encrypt, sizes
    local/          # Flux entry point; values: *.localhost, mkcert CA, throwaway secrets
  images.lock       # every pinned image and chart, read by local-prefetch and CI
  ```

  Apps depend on the platform only through Flux `dependsOn` names, so `infra/` and `platform/` can move to a homelab repo by changing one source and path.
- **`wallet-accountant-ui`** is not touched. Its ticket 07 is answered by ADR-0021 and `docs/architecture/ui-contract.md`; the user closes it.

### Production

- **Proxmox**: the host exists. OpenTofu (`bpg/proxmox`) creates new resources only, and refers to the host's storage pools and bridges by name through variables and data sources. Nothing is imported. A Proxmox API user and token with the least privileges needed are created by hand once and stored in 1Password.
- **VM**: one k3s server VM with 4 cores and 16 GB, installed by cloud-init (k3s and Tailscale), no Ansible. A node-count variable allows agent VMs later.
- **OpenTofu**, not Terraform, for its licence and built-in state encryption. The `bootstrap` stack (Garage) keeps local encrypted state, stored in 1Password. The `homelab` stack keeps its state in Garage's S3 API with lockfile locking. Secrets are read through the 1Password provider with a **read-only service account** limited to a **dedicated vault** (still to be checked against the user's plan). Anything written back, such as the bootstrap state and Garage's keys, is a manual runbook step (ticket 10).
- **Garage** runs in its own LXC on the second RAID array, outside k3s. Remote Garage nodes come later. Replication mirrors deletions, so it is not versioned.
- **Network**: `*.shire.migmar.eu` is a wildcard A record at OVHcloud pointing to the VM's Tailscale IP, managed by OpenTofu. TLS is a wildcard certificate from Let's Encrypt through DNS-01 with the OVH cert-manager webhook. Traefik (shipped with k3s) uses Gateway API.
- **Hostnames**: `wallet`, `auth` (Zitadel), `grafana`, `headlamp`, all under `shire.migmar.eu`.
- **Secrets**: SOPS with age. The age key, Zitadel masterkey, backup credentials, Proxmox token, OVH API keys, Tailscale API key and bootstrap state go in 1Password, plus a printed copy of the age key.
- **Alerts**: Flux notification-controller sends to an ntfy.sh topic with an unguessable name. Flux has no native ntfy provider, so the generic webhook is used; check that the message is readable, and add a small transform if it isn't.

### Platform

- cert-manager, using the OVH webhook in production and the mkcert CA issuer locally.
- CloudNativePG: `zitadel-db`, and `apps-db` with a tenants list (database plus owner role per app). Backups to Garage in production only (ADR-0023).
- Zitadel: one instance and one org, self-registration off, SMTP none for now (an admin resets passwords in the console) (ADR-0022).
- Grafana stack: Prometheus, Loki, Tempo, Grafana. On in production; opt-in locally.
- Headlamp with the Flux plugin. Single sign-on through Zitadel for Grafana and Headlamp, with break-glass logins kept (ADR-0022).

### The chart

As in ADR-0020: one `all` Deployment, the migrate hook Job, the collector, HTTPRoutes, the UI slot, `database.mode`, and the optional Zitadel setup Job. The Job ports `deploy/zitadel/setup-project.sh` and adds the `web` app and 15-minute access tokens. `bootstrap` and `rebuild` run as one-off pods from Make targets that use the chart's pod spec.

### Local

As in ADR-0019:
- k3d with its own registry, whose storage persists outside the cluster;
- an mkcert CA and a CoreDNS rewrite of `auth.localhost` to the ingress;
- Flux pulling an OCI artifact of the working tree;
- `apps-db` (port 5432) and the collector (4318) published on `localhost` for debug mode;
- throwaway local secrets committed in plain text.

### API

All paths move under `/api` (ADR-0021). The OpenAPI `servers` entry is `/api`; the Bruno collection is regenerated.

### CI

On every PR in this repo:
- create a k3d cluster online (no prefetch);
- deploy the PR's image and chart through the same `local-*` path;
- run the smoke test: bootstrap, redeem the code as a real Zitadel identity, call `/api/me`, check the code reached no log;
- run the stand-in UI's sign-in round trip;
- run `helm lint` and the `images.lock` coverage check.

This replaces `smoke.yaml` and `deploy/selfhosted/smoke.sh`. If it is too slow, move it to `main` plus nightly.

### Removing Compose

Last step:
- delete `compose.yaml`, `compose.dev.yaml`, `deploy/selfhosted/` and `deploy/dev/`, and the Make targets that use them;
- move Swagger UI into the local platform at `swagger.localhost`;
- update the README and `docs/architecture/conventions.md`.

## Testing Decisions

- The chart: `helm lint` and `helm template` with each values combination in CI: managed and external database, Zitadel setup on and off, UI on and off.
- End to end: the k3d smoke test above is the main test. It is the same path a developer runs, so a passing PR means `make local-up` works.
- The stand-in UI is a test fixture in this repo, `deploy/helm/wallet-accountant/test/stand-in-ui/`: stock nginx with one page that signs in with oidc-client-ts from a CDN-free bundle and calls `/api/me`. It is never shipped. It proves the `web` app, the redirect URIs, the token's audience and same-origin routing before the real UI exists.
- Backups: a restore drill as part of ticket 12. Restore `apps-db` to a point in time into a scratch cluster, and restore one database from its dump. Write down how long each took.
- Offline: ticket 06 is done when, with the network off on the Mac, `make local-down && make local-up && make local-deploy` succeeds and the smoke test passes.

## Out of Scope

- Building the UI (its own effort, in its repo).
- Importing existing Proxmox VMs or host settings into OpenTofu.
- Exposing anything to the public internet.
- High availability: one node, one replica, one PostgreSQL instance per cluster.
- SMTP for Zitadel.
- Object versioning or immutable backups in Garage.
- Splitting `api` and `worker` into separate Deployments (the chart allows it later).

## Further Notes

- The OVH cert-manager webhook and the Flux-to-ntfy path are community or generic integrations; check how well each is maintained when building.
- CNPG's barman-cloud plugin must be verified against Garage's S3 API, through the restore drill, not assumed.
- Ticket 03 of the UI repo listed points to verify. The stand-in UI covers them: the token's `aud` contains `wallet-accountant`; `CreateApplication` accepts a User Agent app; Zitadel's lifetime settings API; whether sign-out ends the Zitadel session. Locally, HTTPS removes the need for Zitadel's dev mode.
