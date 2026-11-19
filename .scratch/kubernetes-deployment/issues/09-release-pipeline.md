# 09: Publish the image and chart on release tags

**What to build:** pushing a semver tag (`v1.2.3`) publishes the image and the chart to GHCR. Merges to `main` publish a `main-<sha>` image for testing; production never follows it. Spec: `../spec.md` ("Production", user story 3); ADR-0020.

**Blocked by:** 02

**Status:** claimed

- [ ] Image `ghcr.io/mumia/wallet-accountant`: `vX.Y.Z` on tags, `main-<sha>` on `main`; multi-arch only if the Proxmox VM needs it (amd64 expected)
- [ ] Chart `oci://ghcr.io/mumia/charts/wallet-accountant`, with version and appVersion from the tag
- [ ] Provenance: build attestations, plus a signature (cosign keyless) that Flux can verify; decide whether the prod cluster enforces it
- [ ] The release process (who tags, what goes in release notes) written in the conventions doc
