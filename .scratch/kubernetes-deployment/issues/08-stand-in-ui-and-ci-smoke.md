# 08: Stand-in UI and the k3d smoke test in CI

**What to build:** a stand-in UI test fixture proves browser sign-in and same-origin routing end to end. A CI workflow runs the whole local path on every PR and replaces the Compose smoke test. Spec: `../spec.md` ("CI", "Testing Decisions"); ADR-0021, `docs/architecture/ui-contract.md`.

**Blocked by:** 05

**Status:** claimed

- [ ] `deploy/helm/wallet-accountant/test/stand-in-ui/`: an image meeting the UI contract (nginx, port 8080, `/healthz`, SPA fallback, reads `/config.json`) with one page that signs in through the `web` app (PKCE) and shows `/api/me`'s response. It is never published
- [ ] A browser test (Playwright, headless) drives the stand-in UI's sign-in against the local Zitadel and checks `/api/me` answers. It also checks that the access token's `aud` contains the app's audience, and that sign-out ends the Zitadel session
- [ ] `make local-smoke`: from nothing, `local-up`, deploy with the stand-in UI, bootstrap, redeem the code as a real Zitadel identity, call `/api/me`, check the code reached no log, run the browser test, then tear everything down
- [ ] `.github/workflows/smoke.yaml` runs `make local-smoke` on every PR (online, no prefetch), with the `images.lock` and `helm lint` checks; it replaces the Compose smoke job
- [ ] If the job takes over ~15 minutes, move it to `main` and nightly, and note that in the conventions doc
