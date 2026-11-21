# 09: Publish the image and chart on release tags

**What to build:** pushing a semver tag (`v1.2.3`) publishes the image and the chart to GHCR. Merges to `main` publish a `main-<sha>` image for testing; production never follows it. Spec: `../spec.md` ("Production", user story 3); ADR-0020.

**Blocked by:** 02

**Status:** resolved

- [ ] Image `ghcr.io/mumia/wallet-accountant`: `vX.Y.Z` on tags, `main-<sha>` on `main`; multi-arch only if the Proxmox VM needs it (amd64 expected)
- [ ] Chart `oci://ghcr.io/mumia/charts/wallet-accountant`, with version and appVersion from the tag
- [ ] Provenance: build attestations, plus a signature (cosign keyless) that Flux can verify; decide whether the prod cluster enforces it
- [ ] The release process (who tags, what goes in release notes) written in the conventions doc

## Comments

- Implemented in `.github/workflows/release.yaml` (push to `main` and tags `v*`): jobs `version` → `image` → `chart` → `github-release`. `deploy/release/version.sh` maps the ref to `image-tag`/`chart`/`chart-version`/`app-version` outputs; its Go tests (`deploy/release/version_test.go`) run in `make test`. Accepted tags: `vMAJOR.MINOR.PATCH` with an optional pre-release; build metadata is refused (`+` is invalid in an OCI tag). `main-<sha>` uses the first 7 characters of the commit.
- appVersion is the image tag including the `v` (`v1.2.3`), chart version is `1.2.3`: the chart's `image.tag` defaults to appVersion, so a chart release runs its own image without a value. Checked locally: `make chart-package CHART_VERSION=1.2.3 APP_VERSION=v1.2.3` then `helm template` renders `ghcr.io/mumia/wallet-accountant:v1.2.3`.
- Image is `linux/amd64` only (Proxmox VM is x86-64; local clusters build from source).
- The workflow refuses a tag that is not on `main` or is lightweight: the annotated tag's message is the upgrade-notes part of the release notes, followed by the artifacts (digests) and GitHub's generated PR list.
- Provenance: `actions/attest` (pushed to GHCR) and `cosign sign` keyless (cosign v3 via cosign-installer; Flux source-controller verifies both cosign v2 and v3 formats) for the image and the chart, `main-<sha>` included.
- Enforcement decision (conventions, "Releases" → "Provenance"): production Flux verifies the **chart** signature (`verify.provider: cosign`, `matchOIDCIdentity` issuer `https://token.actions.githubusercontent.com`, subject `^https://github\.com/mumia/wallet-accountant/\.github/workflows/release\.yaml@refs/tags/v.*$`), and the env repo leaves `image.tag` empty so the verified chart picks the image. Image signatures are **not** enforced at admission (would need Kyverno/policy-controller on a single node). Ticket 11 configures the Flux side.
- New Make targets: `chart-package`, `chart-push`, `lint-workflows` (actionlint v1.7.12 via `go run`, now part of `make lint`), plus a hidden `print-helm`.
- Not verifiable here: the workflow never ran for real (no pushes to GHCR, no tags). In particular unverified: `helm push` output format (`Digest: sha256:…` parsed for the digest), helm `registry login` path, `gh release create --notes-file` combined with `--generate-notes` prepending the file. One manual step after the first release: make both GHCR packages public.
