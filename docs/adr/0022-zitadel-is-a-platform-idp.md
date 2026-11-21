# Zitadel is a platform identity provider, outside the app's deployment

Zitadel runs as part of the homelab **platform**, not as part of Wallet Accountant. It is one instance with one organisation, where each app is a project. It also signs operators in to Grafana and Headlamp. Self-registration is off; the app still never relies on that (ADR-0017). Zitadel has its own PostgreSQL cluster (ADR-0023).

The Wallet Accountant chart does not install Zitadel. Its optional setup hook Job (ADR-0020), which replaces `deploy/zitadel/setup-project.sh`'s Compose service, uses a personal access token from a Secret to create or update:

- the project whose ID is the app's audience (`WA_OIDC_AUDIENCE`);
- `api-clients`, the public app the Bruno collection signs in with;
- `web`, a public User Agent app with PKCE and refresh tokens for the browser UI, with redirect and post-logout URIs from values;
- 15-minute access tokens.

Self-hosters with another OIDC provider turn the Job off and pass the issuer, audience and client IDs as values (ADR-0010).

We chose a shared IdP because the homelab's other apps will sign in through it too. We rejected installing Zitadel inside the app's chart, because then every app would bring its own IdP.

## Break-glass

Single sign-on for Grafana and Headlamp makes them depend on Zitadel. When Zitadel is down, access goes through these, all stored in 1Password: `kubectl` with the admin kubeconfig over Tailscale, Grafana's local admin, Headlamp's token login, and Zitadel's own first-instance admin.

This amends ADR-0010: Zitadel is no longer shipped by Compose; it is a platform service.
