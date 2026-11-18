# The app ships as a Helm chart; environments live in a separate GitOps repo reconciled by Flux

This repo owns Wallet Accountant's **package**: a Helm chart in `deploy/helm/wallet-accountant`, published with the container image to GHCR (the chart as an OCI artifact). The chart encodes what only this repo knows: the run modes (ADR-0009), the environment variables, the migrate step, the routes. **Environments** (production, local) are not in this repo. They live in a separate, private GitOps repo, `wallet-accountant-deploy`, which pins chart and image versions and holds each environment's values and SOPS-encrypted secrets. Flux reconciles each cluster from that repo.

We chose Helm over Kustomize because Kubernetes is now the self-hosting path (ADR-0019): other self-hosters need one installable artifact, `helm install` with a values file is the standard one, and hooks run `migrate` before every upgrade. We chose Flux over Argo CD because it runs in the cluster with a small footprint, handles Helm charts and Kustomize alike, and decrypts SOPS natively. Headlamp, with its Flux plugin, gives a web view of the cluster, and Flux alerts go to ntfy. We rejected keeping environments in this repo, because homelab details (domain, node names, secrets) would then live in the app's repo.

## The chart

- One Deployment running `all`, one replica. Values can split it into `api` and `worker` Deployments later.
- `migrate` as a `pre-install` / `pre-upgrade` hook Job, with the owner database credentials only in that Job.
- An OpenTelemetry Collector whose exporters come from values (ADR-0016).
- An HTTPRoute (Gateway API) per host: `/api` to the app (ADR-0021), everything else to the UI.
- A **UI slot**, off by default (`ui.enabled`), for an image that meets `docs/architecture/ui-contract.md`. The chart mounts the UI's runtime `config.json` from a ConfigMap.
- `database.mode`: `external` (connection Secrets passed in, as in the homelab, ADR-0023) or `managed` (the chart creates its own CloudNativePG cluster).
- An optional Zitadel setup hook Job (`oidc.zitadelSetup.enabled`, ADR-0022). Without it, the issuer, audience and client IDs come from values, so any OIDC provider works (ADR-0010).

## Releases

CI publishes the image and chart for every semver tag. Production follows tags only: Flux image automation commits the new version to `wallet-accountant-deploy`. Forward-only migrations (ADR-0009) make every deploy one-way, so shipping is a deliberate act. Renovate keeps pinned versions current in both repos.
