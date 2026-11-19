# 13: Renovate in both repos

**What to build:** the Mend Renovate GitHub App (free) keeps every pinned version current in this repo and the env repo, with grouped PRs, replacing Dependabot. Spec: `../spec.md` (user story 9); ADR-0020.

**Blocked by:** 02, 03

**Status:** claimed

- [ ] `renovate.json` here: Go modules, GitHub Actions, the Dockerfile, the chart's dependencies. Dependabot config removed
- [ ] `renovate.json` in the env repo: Flux HelmReleases and OCIRepositories, images in values, `images.lock` (a regex manager), k3s and Tailscale versions in cloud-init, OpenTofu providers
- [ ] Grouping (platform charts together, the Grafana stack together); automerge only for patch versions of platform charts and only when CI passes
- [ ] `images.lock` digests are updated together with the versions that use them
