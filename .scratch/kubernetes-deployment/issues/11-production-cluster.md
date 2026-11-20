# 11: The production environment

**What to build:** `clusters/prod` reconciles the platform and the app at `https://wallet.shire.migmar.eu`, with a Let's Encrypt wildcard certificate, SOPS secrets, image automation following release tags, and ntfy alerts. Spec: `../spec.md` ("Production", user stories 2–8).

**Blocked by:** 05, 07, 09, 10

**Status:** claimed

- [ ] cert-manager with the OVH DNS-01 webhook (check its maintenance first) and a wildcard certificate for `*.shire.migmar.eu`; staging issuer first, then production
- [ ] `.sops.yaml` with the age recipient; every prod Secret encrypted. The list of what must also be in 1Password is in the env repo's README
- [ ] Prod values: hostnames, sizes for 16 GB, the Grafana stack on, Zitadel at `auth.shire.migmar.eu`, the app's chart and image from GHCR
- [ ] Flux image automation: `ImageRepository` and `ImagePolicy` (semver) for the image and the chart; `ImageUpdateAutomation` commits to `main` of the env repo
- [ ] Flux alerts to an ntfy.sh topic with an unguessable name through the generic webhook; check the message is readable, and add a transform if not
- [ ] First run: bootstrap the Instance administrator with `make prod-bootstrap` (kubeconfig over Tailscale) and redeem it from Bruno

## Comments

- 2026-11-18 (orchestrator): the user asked for this ticket to be written as unapplied code in this PR. The agent writes and validates the code (`tofu validate`, `flux build`/`kustomize build`, dry runs) but applies nothing and uses no real credentials. Applying it, first runs and the destroy-and-recreate check stay with the user; after merge the ticket returns to `ready-for-human` for those steps.
