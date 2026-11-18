# 03: The env repo and the local cluster's core

**What to build:** the `wallet-accountant-deploy` repo, scaffolded at `../wallet-accountant-deploy`, with the layout from the spec. `make local-up` (run from this repo, driving the env repo) creates a k3d cluster with its own registry. Flux then reconciles `clusters/local` from an OCI artifact pushed to that registry, and installs cert-manager with an mkcert CA issuer, a Traefik Gateway with a wildcard `*.localhost` certificate, and Headlamp with the Flux plugin at `https://headlamp.localhost`. Spec: `../spec.md` ("Repos", "Local"); ADR-0019, ADR-0020.

**Blocked by:** None (can start immediately)

**Status:** claimed

- [ ] Scaffold `infra/`, `platform/`, `apps/`, `clusters/{local,prod}`, `images.lock` and the README in `../wallet-accountant-deploy`. The user has already created it with its private GitHub remote (`mumia/wallet-accountant-deploy`, initial commit with a README and `.gitignore`). Build on that commit and ask before pushing
- [ ] `platform/` has no reference to any app; `apps/` reaches it only through Flux `dependsOn`
- [ ] k3d config: one server, ports 80 and 443 on the host, a k3d-managed registry whose storage is a host directory that survives `local-down`
- [ ] `make local-up` / `local-down` / `local-deploy` in this repo; `local-deploy` runs `flux push artifact` with the env repo's working tree to the local registry, and Flux's `OCIRepository` follows it
- [ ] mkcert: a Make target installs the CA on the Mac (once) and loads it as the cert-manager CA issuer's Secret
- [ ] Traefik with Gateway API enabled; one Gateway with an HTTPS listener for `*.localhost`
- [ ] CoreDNS rewrite: `auth.localhost` (and the rest of `*.localhost` used by the platform) resolves to Traefik inside the cluster
- [ ] Headlamp at `https://headlamp.localhost` with the Flux plugin, token login for now
- [ ] Throwaway local secrets committed in plain text under `clusters/local`, clearly marked
