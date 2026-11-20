# UI contract

What the Wallet Accountant UI image must do so the Helm chart (ADR-0020) can serve it next to the API on one origin (ADR-0021). The UI lives in its own repo (`wallet-accountant-ui`). This document is the backend's side of that agreement. The chart's UI slot is off (`ui.enabled: false`) until an image meets it.

## The image

- Serves the built static files over plain HTTP on port **8080**, as a **non-root** user, with a read-only root filesystem. TLS ends at the gateway. The chart mounts an empty, writable **`/tmp`**: the server's pid and temporary files go there (`nginxinc/nginx-unprivileged` does this already).
- Answers **`GET /healthz`** with 200 and no auth, for the chart's probes.
- **SPA fallback**: any `GET` for a path that is not a file returns `index.html` with 200, so deep links work.
- Caching: hashed assets (`/assets/*`) get `Cache-Control: public, max-age=31536000, immutable`; `index.html` and `config.json` get `no-cache`.
- Has no runtime other than a static file server (no Node at runtime). Caddy or nginx are expected.

## Paths

- The UI owns **every path except `/api` and `/api/*`**. The gateway sends those to the app before the UI sees them.
- Calls to the API use relative URLs under `/api`, from the same origin. There is no CORS.

## Runtime configuration

The same image runs in every environment. Nothing is baked in at build time. The UI loads **`/config.json`** before starting. The chart mounts it from a ConfigMap over the image's own copy, at the path set by `ui.configPath` (default `/srv/config.json`; the image documents its path).

```json
{
  "apiBaseUrl": "/api",
  "oidc": {
    "issuer": "https://auth.shire.migmar.eu",
    "clientId": "<the web app's client ID>",
    "audience": "wallet-accountant",
    "redirectPath": "/auth/callback",
    "postLogoutRedirectPath": "/"
  }
}
```

- `issuer`, `clientId` and `audience` come from the chart's values. When the Zitadel setup Job runs (ADR-0022), the client ID is the one it created for the `web` app.
- `redirectPath` and `postLogoutRedirectPath` are relative to the UI's origin. The chart registers `https://<host><path>` with Zitadel for each.
- New keys may be added; the UI must ignore keys it does not know.

## Sign-in

- Authorization code with PKCE, as a public client (no secret), with refresh tokens, through the `web` app.
- Access tokens last 15 minutes, and their audience includes the API's audience.

## Versioning

The UI image is versioned and released separately from the backend. The chart pins a UI image tag in values, and the environment repo bumps it like the app image. A UI release states the lowest API contract version it needs.

## The stand-in UI

`deploy/helm/wallet-accountant/test/stand-in-ui` is a test fixture that meets this contract with stock nginx and one page (oidc-client-ts, vendored, no CDN). It is never published. `make local-smoke` deploys it in the UI slot, and its Playwright test checks, against the local Zitadel: `/healthz`, the SPA fallback, `config.json`, `/api` reaching the app on the same origin, sign-in with PKCE through the `web` app, an access token whose `aud` contains the API's audience, and that signing out ends the Zitadel session (a `prompt=none` sign-in that succeeded before signing out fails afterwards; Zitadel answers `interaction_required`). The real UI can read it as a working example.
