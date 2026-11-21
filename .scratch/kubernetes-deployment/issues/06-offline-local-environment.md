# 06: The local environment works offline

**What to build:** after one online `make local-prefetch`, the local cluster starts, deploys and runs with the Mac's network off. Spec: `../spec.md` (user story 11); ADR-0019.

**Blocked by:** 05

**Status:** resolved

- [ ] `images.lock` in the env repo lists every image (by digest) and every Helm chart (by version) that `clusters/local` uses: k3s, Flux, cert-manager, Traefik, CNPG, Zitadel, the Grafana stack, Headlamp, Swagger UI, the collector, plus the base images this repo's Dockerfile and the stand-in UI use
- [ ] `make local-prefetch` mirrors them into the k3d registry; Helm charts are mirrored as OCI artifacts and HelmReleases use them through `OCIRepository` or a local `HelmRepository`
- [ ] k3s's `registries.yaml` mirrors every upstream registry to the local one, so manifests keep upstream image names
- [ ] The Go build works offline (module cache mounted into the build, or a vendored build for the local image); verify it
- [ ] A check, run in CI, fails when a rendered manifest references an image or chart missing from `images.lock`
- [ ] Done when, with the network off: `make local-down && make local-up && make local-deploy`, then a bootstrap and a call to `/api/me` from Bruno all work

## Comments

**Implementer (2026-11-20).** Built in this repo (`deploy/local/prefetch.sh`, `deploy/local/imageslock`, `make local-prefetch`, `images-lock-check`, `images-lock-pin`, offline checks in `local-check`, CI job `images-lock`) and the env repo (`images.lock` with digests and host images, `k3d.yaml` mirrors and `--disable-default-registry-endpoint`, `clusters/local/offline` Component for charts, `registry-config.yml`).

What was verified, online:
- `make local-down && make local-up && make local-deploy && make local-check` from a fresh cluster, with the registry storage already prefetched: `local-up`'s prefetch only checked (every entry `ok`, no pull), and `local-check` passed, including the bootstrap, redeeming the code as a Zitadel identity and `/api/me`, plus the offline checks: the node's images are all in `images.lock`, every `images.lock` entry is in `wa-registry` at its digest, every Flux source fetches from `wa-registry:5000`, and pulls of images that exist upstream but were not prefetched (docker.io, ghcr.io, quay.io, registry.k8s.io, and gcr.io through the `"*"` mirror) fail with "not found" after reaching only `wa-registry` (its access log). So nothing the cluster runs comes from upstream, even with the network up.
- The app image builds with `docker build --network=none` from vendored modules (`go mod vendor` with `GOPROXY=off`, from the module cache).
- `make images-lock-check` fails on a chart or image missing from `images.lock` (tried by removing one from a copy of the env repo) and on an upstream chart source or an unmirrored registry.

Found and fixed during the live run: k3d expands `$1` in its config as an environment variable, so the mirrors' rewrite `"^(.*)$": "docker.io/$1"` reached k3s as `docker.io/` and every pull 404'd. The rewrite is now `"^": "docker.io/"`, and `images-lock-check` rejects any `$` in a rewrite.

Left for you:
- **The network-off run.** Turn the Mac's network off, then `make local-down && make local-up && make local-deploy`, `make local-bootstrap`, and call `/api/me` from Bruno. What is not proven without that: Docker resolving the Dockerfile's `FROM` images and k3d's own images from the local image store with no network (they are pulled and tagged by `local-prefetch`), and `flux`/`helm` not reaching out.
- **CI secret.** The `images-lock` job in `.github/workflows/ci.yaml` checks out the private env repo with the read-only deploy key in the secret `ENV_REPO_DEPLOY_KEY` (and the variable `ENV_REPO_REF`), the same mechanism as ticket 08's smoke workflow (unified when 08 merged; this ticket first used a fine-grained token, `ENV_REPO_TOKEN`, which is no longer needed). Until that secret exists, the job fails at checkout.
- `make local-check`'s bootstrap checks only pass on a fresh cluster (the bootstrap Invitation is single-use); that was already so after ticket 05.
- The stand-in UI's base image is not in `images.lock` yet: the stand-in UI is ticket 08's. Add its Dockerfile to `LOCK_DOCKERFILES` in the Makefile and its base image to `images.lock` then.
