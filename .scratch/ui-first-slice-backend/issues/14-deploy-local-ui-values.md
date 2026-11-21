# 14: `wallet-accountant-deploy`: local UI values

**What to build:** plain config in the environment repo `../wallet-accountant-deploy` for the UI, decided in UI ticket 07. It needs no grilling. Spec: `../spec.md`, "Further Notes".

**Blocked by:** the UI image existing (UI repo); the redirect URIs don't need it

**Status:** ready-for-human

- [ ] Add `https://localhost:5173/auth/callback` to `oidc.zitadelSetup.web.extraRedirectUris` and `https://localhost:5173/` to `extraPostLogoutRedirectUris` (Vite dev server)
- [ ] Set `ui.enabled: true` with the UI image and tag, once the image exists
- [ ] Add the UI image's base image (Caddy) to `images.lock`

## Comments
