# The API is served under `/api`, on the same origin as the UI

The API's paths start with `/api` (`/api/me`, `/api/households`, …), and the OpenAPI document's `servers` entry is `/api`. The UI and the API share one origin, such as `https://wallet.shire.migmar.eu`. The router sends `/api/*` to the app and every other path to the UI's static image. The browser therefore never makes a cross-origin call, the API needs no CORS, and sign-in redirect URIs live on one host.

We put the prefix in the API itself rather than having a proxy strip it. Then a plain path-prefix route works with any Gateway API implementation, the Vite dev server's proxy needs no rewrite, and production, local and development all route the same way. We rejected two alternatives. Serving the UI from the Go binary would tie the two repos' releases together. A separate UI origin would need CORS and a second set of redirect URIs.

The UI image's obligations (static files, its runtime config, which paths it owns) are in `docs/architecture/ui-contract.md`. This amends ADR-0008: the contract's paths now carry the `/api` prefix.
