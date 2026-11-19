# 13: Renovate in both repos

**What to build:** the Mend Renovate GitHub App (free) keeps every pinned version current in this repo and the env repo, with grouped PRs, replacing Dependabot. Spec: `../spec.md` (user story 9); ADR-0020.

**Blocked by:** 02, 03

**Status:** resolved

- [ ] `renovate.json` here: Go modules, GitHub Actions, the Dockerfile, the chart's dependencies. Dependabot config removed
- [ ] `renovate.json` in the env repo: Flux HelmReleases and OCIRepositories, images in values, `images.lock` (a regex manager), k3s and Tailscale versions in cloud-init, OpenTofu providers
- [ ] Grouping (platform charts together, the Grafana stack together); automerge only for patch versions of platform charts and only when CI passes
- [ ] `images.lock` digests are updated together with the versions that use them

## Comments

**Implementer (2026-11-19).** wallet-accountant branch `worktree-agent-a1a081b31eafac469`; env repo branch `ticket-13` (worktree `/Users/mumia/development/wad-worktrees/ticket-13`). Neither pushed.

- Here: `renovate.json` (`config:recommended`, weekly, Action digests pinned, `gomodTidy`), `.github/dependabot.yml` removed. Managers: gomod, github-actions, dockerfile, helm-values (the chart's `repository`/`tag` images; the chart has no `dependencies`), docker-compose (until ticket 14), and a regex manager for `# renovate: datasource=<d> depName=<name>` comments, used for the Makefile's helm/kubeconform and govulncheck in CI (moved into a `GOVULNCHECK_VERSION` env). Groups: Go modules (non-major), Go toolchain (go.mod + `golang` image), GitHub Actions, chart images, Makefile tools. `docs/architecture/conventions.md` now names Renovate.
- Env repo: `renovate.json` plus a "Renovate" section in the README listing the conventions later tickets must follow (Flux sources and `image:` lines under `platform|apps|clusters`, `images.lock` line formats, `.tf`/`.tofu` providers under `infra/`, and the `# renovate:` comment for cloud-init's k3s/Tailscale and anything else). Groups: platform charts (Flux `HelmChart` deps under `platform/` + `images.lock` charts), images (manifests, HelmRelease values, `images.lock` images), k3s (k3d image + `k3s-io/k3s` releases, regex versioning on the `k3sN` build), Tailscale, Grafana stack, OpenTofu providers. Patch updates of platform charts automerge (`automergeType: pr`), which Renovate only does on green checks: until the env repo has CI checks on PRs (spec, "CI"), nothing merges itself. Wallet Accountant's own chart/image are disabled (Flux image automation, ADR-0020); `wa-registry`/`localhost` are disabled.
- `images.lock` digests: an image line `- <name>:<tag>@sha256:<digest>` is captured with `currentDigest`, so Renovate replaces tag and digest together; chart and image entries use the same names as the manifests, so both land in one branch (verified: `rancher/k3s` in `k3d.yaml` and `images.lock` → `renovate/k3s`; cert-manager OCIRepository + `images.lock` → `renovate/platform-charts`).
- Verification: `make renovate-check` (`deploy/renovate/check.sh`, Docker only, no Node): `renovate-config-validator --strict` on each repo, then `renovate --platform=local --dry-run=lookup`, failing when an expected `manager:depName` is missing. Passes for both repos. The cloud-init / OpenTofu / digest conventions were checked on a scratch copy of the env repo with sample files (k3s and Tailscale `# renovate:` lines, `bpg/proxmox`, a digest-pinned image, a values `tag:`), plus a `--dry-run=full` to see the branch grouping. GitHub-hosted lookups (Actions, `github-releases`) report `github-token-required` locally; set `GITHUB_COM_TOKEN` to see them. Automerge is configured but can only be observed on the real platform.
- Note: the flux manager also finds images in HelmRelease values, as does the kubernetes manager; both are grouped into the same `images` branch.
- **User step:** install the Mend Renovate GitHub App on `mumia/wallet-accountant` and `mumia/wallet-accountant-deploy` (it reads the committed `renovate.json`, so skip its onboarding PR or close it), and turn off Dependabot version updates in the repo settings if they were enabled there.
