# 06: The local environment works offline

**What to build:** after one online `make local-prefetch`, the local cluster starts, deploys and runs with the Mac's network off. Spec: `../spec.md` (user story 11); ADR-0019.

**Blocked by:** 05

**Status:** claimed

- [ ] `images.lock` in the env repo lists every image (by digest) and every Helm chart (by version) that `clusters/local` uses: k3s, Flux, cert-manager, Traefik, CNPG, Zitadel, the Grafana stack, Headlamp, Swagger UI, the collector, plus the base images this repo's Dockerfile and the stand-in UI use
- [ ] `make local-prefetch` mirrors them into the k3d registry; Helm charts are mirrored as OCI artifacts and HelmReleases use them through `OCIRepository` or a local `HelmRepository`
- [ ] k3s's `registries.yaml` mirrors every upstream registry to the local one, so manifests keep upstream image names
- [ ] The Go build works offline (module cache mounted into the build, or a vendored build for the local image); verify it
- [ ] A check, run in CI, fails when a rendered manifest references an image or chart missing from `images.lock`
- [ ] Done when, with the network off: `make local-down && make local-up && make local-deploy`, then a bootstrap and a call to `/api/me` from Bruno all work
