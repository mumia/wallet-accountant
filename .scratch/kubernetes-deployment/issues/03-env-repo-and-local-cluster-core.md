# 03: The env repo and the local cluster's core

**What to build:** the `wallet-accountant-deploy` repo, scaffolded at `../wallet-accountant-deploy`, with the layout from the spec. `make local-up` (run from this repo, driving the env repo) creates a k3d cluster with its own registry. Flux then reconciles `clusters/local` from an OCI artifact pushed to that registry, and installs cert-manager with an mkcert CA issuer, a Traefik Gateway with a wildcard `*.localhost` certificate, and Headlamp with the Flux plugin at `https://headlamp.localhost`. Spec: `../spec.md` ("Repos", "Local"); ADR-0019, ADR-0020.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [ ] Scaffold `infra/`, `platform/`, `apps/`, `clusters/{local,prod}`, `images.lock` and the README in `../wallet-accountant-deploy`. The user has already created it with its private GitHub remote (`mumia/wallet-accountant-deploy`, initial commit with a README and `.gitignore`). Build on that commit and ask before pushing
- [ ] `platform/` has no reference to any app; `apps/` reaches it only through Flux `dependsOn`
- [ ] k3d config: one server, ports 80 and 443 on the host, a k3d-managed registry whose storage is a host directory that survives `local-down`
- [ ] `make local-up` / `local-down` / `local-deploy` in this repo; `local-deploy` runs `flux push artifact` with the env repo's working tree to the local registry, and Flux's `OCIRepository` follows it
- [ ] mkcert: a Make target installs the CA on the Mac (once) and loads it as the cert-manager CA issuer's Secret
- [ ] Traefik with Gateway API enabled; one Gateway with an HTTPS listener for `*.localhost`
- [ ] CoreDNS rewrite: `auth.localhost` (and the rest of `*.localhost` used by the platform) resolves to Traefik inside the cluster
- [ ] Headlamp at `https://headlamp.localhost` with the Flux plugin, token login for now
- [ ] Throwaway local secrets committed in plain text under `clusters/local`, clearly marked

## Comments

**Implementer (2026-11-19).** Env repo branch `ticket-03` (not pushed). Verified with `make local-up` from scratch, `make local-deploy` of an uncommitted change, and `make local-check` (`deploy/local/check.sh`, the ticket's test against the running cluster).

- `ENV_REPO ?= ../wallet-accountant-deploy` points the `local-*` targets at any env-repo checkout.
- Host ports: on the machine this was built on, 80 and 443 were held by another (kind) cluster, so `LOCAL_HTTP_PORT` / `LOCAL_HTTPS_PORT` (default 80 / 443, expanded by k3d in `clusters/local/k3d.yaml`) exist; it was verified on 18080 / 18443. With the defaults, `https://headlamp.localhost` needs no port.
- The registry is `wa-registry` (host `localhost:5050`, in-cluster `wa-registry:5000`), storage in `LOCAL_REGISTRY_DIR` (default `~/.local/share/wallet-accountant/registry`); it survived `local-down`.
- mkcert: `make local-ca` runs `mkcert -install` (needs the user's password; run once). `make local-up` loads `$(mkcert -CAROOT)`'s CA into `cert-manager/mkcert-ca`, so the CA key is never committed.
- Traefik is k3s's own, configured with a `HelmChartConfig` (`platform/traefik`): Gateway provider on, chart's default Gateway off, HTTP redirected to HTTPS. The Gateway is `gateway/platform`, listener `https` on `*.${DOMAIN}`. Because some browsers refuse a bare `*.localhost` wildcard, the local Certificate also names auth, headlamp, grafana, swagger and wallet explicitly (patch in `clusters/local/platform.yaml`).
- CoreDNS: the `coredns-custom` ConfigMap rewrites every `*.localhost` to `traefik.kube-system.svc.cluster.local`.
- Headlamp's Flux plugin comes from the `ghcr.io/headlamp-k8s/headlamp-plugin-flux` image through an init container (no Node.js, and the image can be prefetched for ticket 06). Token: `make local-headlamp-token`.
- Throwaway secrets: `clusters/local/secrets/` exists with its marking rules (README, header line checked by `local-check`), but holds no secret yet: nothing in this ticket needs one. Ticket 04's Zitadel masterkey and database passwords are the first.
- `images.lock` lists the charts and top-level images pinned so far; digests and chart-pulled images are ticket 06.
- Caveat: when the Docker VM is starved (many concurrent testcontainers), k3s's cloud-controller-manager can exit before RBAC is ready and k3s then restarts in a loop. `make local-down && make local-up` recovers once the load is gone.
