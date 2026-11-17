# Bruno collection

A [Bruno](https://www.usebruno.com) collection for the Wallet Accountant API, in the OpenCollection YAML format (Bruno 3.0 or later).

`make generate` writes it from `api/openapi.yaml` (`internal/tools/opencollection`). It writes `opencollection.yml` and one folder per tag, and overwrites them on every run, so change the contract rather than the requests. CI fails when the collection is out of date with the contract. This README, `.gitignore`, `.env.sample` and `.env` are not generated.

## Using it

1. Run `make dev-up`, `make migrate` and `make run`. `dev-up` creates the public Zitadel application `api-clients` and writes its client ID to `.env` in this directory as `WA_OAUTH_CLIENT_ID`. The client ID changes after `make dev-reset`; `make dev-up` writes the new one.
2. In Bruno, open this directory (*Open Collection*). No environment is needed: the collection's variables (`baseUrl`, `authUrl`, `clientId` and the IDs) are set for the development setup.
3. Send any request. Bruno opens the Zitadel login, signs in with the authorization code flow and PKCE, and puts the access token in `Authorization: Bearer …`. It refreshes the token by itself (`offline_access`). The development login is `admin@zitadel.localhost` with the password `Password1!`.
4. A new identity is not a User yet: `GET /me` answers 403 `/problems/registration-required`. Get an Invitation code with `make dev-bootstrap` and send *Users → Redeem an Invitation* with it.

Requests that create something (a Household, an Account, a Connection, …) store the new ID in the runtime variable of the same name, such as `household_id`, which the requests under it use in their paths. Every POST sends a fresh `Idempotency-Key`.

## Signing in

The collection's authentication, which every folder and request inherits, is OAuth 2.0 authorization code with PKCE:

| Setting | Value |
|---|---|
| Authorization URL | `{{authUrl}}/oauth/v2/authorize` |
| Token and refresh URL | `{{authUrl}}/oauth/v2/token` |
| Client ID | `{{clientId}}`, which is `{{process.env.WA_OAUTH_CLIENT_ID}}`; a public client, with no secret |
| Callback URL | `https://oauth.usebruno.com/callback` |
| Scope | `openid profile email offline_access` |

Zitadel issues the access token as a JWT whose audience includes the project ID `wallet-accountant`, which is what the app checks (`WA_OIDC_AUDIENCE`).

Bruno's own login window picks up the callback. If you have turned on *Preferences → Use System Browser for OAuth2*, the callback page `https://oauth.usebruno.com/callback` passes the code back to Bruno. `api-clients` also allows `http://localhost/oauth/callback` on any port, for the local callback server `npx @usebruno/oauth2-callback-server`; to use it, change the callback URL in the collection's Auth tab.

Against another provider or instance, copy `.env.sample` to `.env` with that provider's client ID, and create a Bruno environment that sets `baseUrl` and `authUrl`; an environment's variables take precedence over the collection's. Bruno saves it under `environments/`, which `make generate` leaves alone. The client must be public (PKCE, no secret), allow the callback URL above, and issue JWT access tokens for the app's audience.
