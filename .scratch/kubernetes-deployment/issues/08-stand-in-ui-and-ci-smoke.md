# 08: Stand-in UI and the k3d smoke test in CI

**What to build:** a stand-in UI test fixture proves browser sign-in and same-origin routing end to end. A CI workflow runs the whole local path on every PR and replaces the Compose smoke test. Spec: `../spec.md` ("CI", "Testing Decisions"); ADR-0021, `docs/architecture/ui-contract.md`.

**Blocked by:** 05

**Status:** claimed

- [ ] `deploy/helm/wallet-accountant/test/stand-in-ui/`: an image meeting the UI contract (nginx, port 8080, `/healthz`, SPA fallback, reads `/config.json`) with one page that signs in through the `web` app (PKCE) and shows `/api/me`'s response. It is never published
- [ ] A browser test (Playwright, headless) drives the stand-in UI's sign-in against the local Zitadel and checks `/api/me` answers. It also checks that the access token's `aud` contains the app's audience, and that sign-out ends the Zitadel session
- [ ] `make local-smoke`: from nothing, `local-up`, deploy with the stand-in UI, bootstrap, redeem the code as a real Zitadel identity, call `/api/me`, check the code reached no log, run the browser test, then tear everything down
- [ ] `.github/workflows/smoke.yaml` runs `make local-smoke` on every PR (online, no prefetch), with the `images.lock` and `helm lint` checks; it replaces the Compose smoke job
- [ ] If the job takes over ~15 minutes, move it to `main` and nightly, and note that in the conventions doc

## Comments

- **User step: CI access to the env repo.** `.github/workflows/smoke.yaml` checks out the private `mumia/wallet-accountant-deploy` with a read-only deploy key. Add a deploy key (read-only) to the env repo and its private key as the secret `ENV_REPO_DEPLOY_KEY` here. The repository variable `ENV_REPO_REF` picks the env repo branch (default `main`). Until then the Smoke job, and CI's `images-lock` job (which uses the same key since this ticket merged), fail at the env repo checkout.
- **Timing.** `make local-smoke` passed from nothing in 479 s (about 8 minutes) on the Mac, with the k3s node pulling every image online. That is under ~15 minutes, so the job runs on every pull request (and on pushes to `main`). It has not run on GitHub's runner yet (it needs the deploy key). If it takes over ~15 minutes there, move it to `main` plus nightly and note that in `docs/architecture/conventions.md` ("CI").
- **images.lock check.** Ticket 06 owns the check that every image and chart is in `images.lock`. At merge, the workflow's placeholder step became `make images-lock-check`, the stand-in UI's Dockerfile joined `LOCK_DOCKERFILES`, and the env repo's `images.lock` lists its base image, `nginxinc/nginx-unprivileged:1.31.7-alpine-slim`, under `host-images` with its digest.
- **Chart change.** With the Zitadel setup on, the oidc ConfigMap is now a `pre-install` hook with empty client IDs, so the UI can start before the post-install setup Job runs (Helm waits for the UI to be ready first). The UI Deployment also gets an emptyDir on `/tmp` for a read-only root filesystem (`docs/architecture/ui-contract.md`).
- **Sign-out.** Zitadel (v4, v1 login) answers a `prompt=none` sign-in after sign-out with `interaction_required`, which the test accepts along with `login_required`. That confirms sign-out ends the Zitadel session.
- **local-check with the stand-in UI.** `make local-check` expects the env repo's HelmRelease as committed (UI off), so after `make local-deploy LOCAL_STAND_IN_UI=1` its "UI off" check fails, as intended. Every other check passed against a cluster deployed with the stand-in UI. After a plain `make local-deploy`, the UI-off check passed too.
