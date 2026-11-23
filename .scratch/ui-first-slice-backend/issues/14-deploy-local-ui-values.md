# 14: `wallet-accountant-deploy`: local UI values

**What to build:** plain config in the environment repo `../wallet-accountant-deploy` for the UI, decided in UI ticket 07. It needs no grilling. Spec: `../spec.md`, "Further Notes".

**Blocked by:** the UI image existing (UI repo); the redirect URIs don't need it

**Status:** resolved

- [x] Add `https://localhost:5173/auth/callback` to `oidc.zitadelSetup.web.extraRedirectUris` and `https://localhost:5173/` to `extraPostLogoutRedirectUris` (Vite dev server)
- [x] Set `ui.enabled: true` with the UI image and tag, once the image exists (locally through `LOCAL_UI_BUILD`, see Comments)
- [x] Add the UI image's base image (Caddy) to `images.lock`

## Comments

- 2026-11-23: The redirect URIs and Caddy's `images.lock` line are in wallet-accountant-deploy#32.
- 2026-11-23: The local cluster doesn't set `ui.enabled` in the environment repo: `make local-push` stages that repo's working tree and never writes to it. Instead, `deploy/local/stage.sh` turns the UI slot on from a build file. wallet-accountant-ui's `make local-deploy` records its image and tag in its own `bin/local-build.env`, which this repo reads as `LOCAL_UI_BUILD`. When the stand-in UI's build (`LOCAL_STAND_IN_UI=1`) is also there, the newer one wins, counting only images still in the registry. `make local-smoke` empties `LOCAL_UI_BUILD`, so it always tests the stand-in. See `docs/architecture/ui-contract.md`, "The real UI on the local cluster". Checked on the local cluster: the real UI image served at `https://wallet.localhost:18443`, with the chart's read-only settings, signing in to `/api/me` with a 200.
