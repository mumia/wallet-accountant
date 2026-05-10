---
status: accepted
date: 2026-05-08
decision-makers: [Miguel Manso]
consulted: []
informed: []
supersedes: null
---

# ADR-005: OAuth 2.0 / OIDC — Zitadel as IdP, local JWT validation, Google federation

## Context and Problem Statement

wallet-accountant authenticates inbound HTTP requests via signed JWTs ([ADR-002](ADR-002-multi-tenant-isolation.md), which pins the `tid` claim as the tenant identity). That ADR named the *transport* (JWT in `Authorization: Bearer …`) but deferred the concrete identity-provider choice and the token model. This ADR closes those.

Three things need answering together — they interact:

1. **Which IdP issues the tokens?** Self-hosted (Zitadel, Keycloak, Ory Hydra) vs managed (Auth0, AWS Cognito).
2. **How does the API validate tokens?** Local JWT validation (signature + claims, against the IdP's JWKS) vs OAuth 2.0 Token Introspection (every request round-trips to the IdP).
3. **How do users authenticate?** Username/password against the IdP only, or federation (Google / Apple / Microsoft) so users can reuse an existing account.

The choices interact: a managed IdP cuts ops cost but undermines [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)'s local-deployment driver (the API would always need to reach the cloud); local validation pairs naturally with self-hosted IdPs and makes the API tolerant of IdP unavailability for ongoing requests; federation is an IdP-side feature, not an API-side feature, so where it lives depends on the IdP.

How should we choose the IdP, the token-validation model, and the federation surface?

## Decision Drivers

- **Local-deployment compatibility (per [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)).** The IdP must run as a container alongside Axon Server, MongoDB, Restate, and the Spring Boot app. No managed-service dependency.
- **Self-hostable, open source.** No vendor lock-in, no per-MAU pricing. A personal project shouldn't carry a SaaS auth bill.
- **Compatible with [ADR-002](ADR-002-multi-tenant-isolation.md)'s `tid` claim.** The IdP must emit a JWT custom claim that carries the tenant identity (mapped from whatever the IdP calls "organization" / "tenant").
- **Federation support — specifically Google.** Users should be able to sign in with their Google account without me hand-rolling the Google OAuth dance.
- **API stays simple.** Local JWT validation only — no introspection round-trips, no shared secret, no key material maintained by the application beyond a JWKS cache.
- **Standards-first.** OIDC, JWKS, RFC-compliant flows. No proprietary token formats.

## Considered Options

**IdP choice:**

1. **Zitadel** — open source, self-hostable, OIDC-first, native external-IDP federation (Google, Apple, GitHub, Microsoft, generic OIDC), Organizations-as-tenants model, active development.
2. **Keycloak** — open source, very mature, Red Hat–backed, OIDC + SAML, realm-as-tenant model, complex setup and heavier resource footprint.
3. **Auth0** — managed SaaS, mature, well-documented, expensive past free tier, requires cloud connectivity.
4. **AWS Cognito** — managed, AWS-locked, limited federation polish, requires AWS account.
5. **Ory Hydra** — headless OAuth2/OIDC server (no UI, no user management), must be paired with Ory Kratos for identity. Great composability but more moving parts than the alternatives.

**Token validation:**

A. **Local JWT validation** — Spring Security resource-server fetches the IdP's JWKS, caches the keys, validates signature + claims locally. Fast, no per-request network hop, tolerates IdP unavailability for already-issued tokens.
B. **Token introspection (RFC 7662)** — every API call POSTs the token to the IdP's introspection endpoint. Simpler revocation, slower, hard dependency on IdP availability.

## Decision Outcome

Chosen combination: **Zitadel as IdP, local JWT validation, federation configured at Zitadel.**

The decision lands as the following hard rules:

- The OAuth 2.0 / OIDC identity provider MUST be Zitadel. Production / staging / local-development environments all authenticate against a Zitadel instance (running as a container per [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)). NEVER integrate Keycloak, Auth0, AWS Cognito, Ory Hydra, or any other IdP into the Spring Boot application.
- The API MUST validate JWTs locally using Spring Security's resource-server with `spring.security.oauth2.resourceserver.jwt.issuer-uri` (or `jwk-set-uri`) pointing at the Zitadel issuer. NEVER use OAuth 2.0 Token Introspection (RFC 7662) — `spring.security.oauth2.resourceserver.opaquetoken.*` properties MUST NOT appear in any `application*.yml`.
- JWTs MUST be signed with RS256 or ES256 (asymmetric). NEVER configure, accept, or fall back to HS256 — symmetric signing requires sharing the signing key with every validator and is incompatible with the local-validation model.
- Federation with external identity providers (Google, Apple, Microsoft, GitHub, generic OIDC) MUST be configured *at the Zitadel side* via Zitadel's external IDPs feature. NEVER integrate `google-oauth-client`, `google-api-services-*`, Apple Sign-In SDKs, MSAL, or any other upstream-IdP client directly from the Spring Boot application. The application MUST only see Zitadel-issued JWTs.
- Every access token MUST carry a `tid` claim populated from Zitadel's Organization ID (configured via a Zitadel Action or claim mapper) per [ADR-002](ADR-002-multi-tenant-isolation.md). The Spring Security `JwtAuthenticationConverter` MUST extract `tid` and populate the request-scoped `TenantContext` referenced in ADR-002.
- Access-token lifetime MUST be ≤ 15 minutes. NEVER configure access-token TTL longer than 15 minutes at Zitadel — local validation has no revocation channel, and longer lifetimes widen the post-revocation attack window unacceptably.
- Refresh tokens MUST rotate on every use: each refresh exchange issues a new refresh token and invalidates the prior one. NEVER configure non-rotating (long-lived static) refresh tokens at Zitadel.
- Refresh tokens MUST have an absolute lifetime ≤ 30 days regardless of rotation, and an inactivity timeout of ≤ 14 days (a refresh token unused for 14 days is revoked). NEVER allow a session chain to extend indefinitely.
- Spring Security's JWT decoder MUST cache Zitadel's JWKS and refresh on `kid` mismatch or after a TTL of ≤ 24 hours. NEVER hardcode public keys, NEVER bypass JWKS validation, and NEVER pin a single `kid` — key rotation at the IdP must Just Work.

### Consequences

**Positive:**
- One IdP container alongside the rest of the stack — `docker compose up` brings Zitadel up too. No cloud dependency for auth.
- Local JWT validation means the API is fast (no per-request network hop) and tolerant of brief Zitadel outages for already-issued tokens.
- Federation is a Zitadel-side configuration change; the Spring Boot app never imports Google/Apple/Microsoft client libraries.
- Standards-first surface (OIDC, JWKS, RFC 7519/7517) — swapping Zitadel for another OIDC-compliant IdP later is a config change, not a code rewrite.
- `tid` claim flows naturally: Zitadel's Organization concept maps cleanly to the tenant model in ADR-002.

**Negative:**
- Local validation has no revocation channel for access tokens. A compromised access token is valid until expiry. Mitigated by the 15-minute access-token lifetime and rotating refresh tokens.
- Zitadel is younger than Keycloak; smaller community, fewer Stack Overflow answers. The open-source release line is active and well-documented, but the long-tail support is thinner.
- Operating the IdP is now part of the project. Backups, version upgrades, and key rotation are local responsibilities.
- A second container's worth of memory on the developer host (Zitadel + its Postgres backing store — Zitadel itself uses Postgres internally; this is an internal-to-Zitadel detail and does NOT count as a wallet-accountant persistence store under [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)).

**Neutral:**
- Federation works only if Google (or any future upstream) is reachable — but that's a property of any federation regardless of IdP choice, and federation is opt-in per user.
- Token-lifetime defaults are tuneable but pinned by directives. Loosening them requires an ADR amendment.

## Pros and Cons of the Options

### IdP — Zitadel (chosen)

- ✅ Self-hostable as a single container; fits [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)'s local-deployment driver.
- ✅ Native external-IDP federation (Google included) configurable from the admin UI — no code in the API.
- ✅ Organizations-as-tenants model maps cleanly to [ADR-002](ADR-002-multi-tenant-isolation.md).
- ✅ Custom claims (e.g., `tid`) supported via Zitadel Actions.
- ❌ Younger and less ubiquitous than Keycloak; smaller community.
- ❌ Internally backed by Postgres — Zitadel runs its own DB; not visible to the wallet-accountant app, but worth knowing operationally.

### IdP — Keycloak

- ✅ Most mature OSS IdP; large community; OIDC + SAML.
- ✅ Realm-as-tenant model also viable.
- ❌ Heavier resource footprint than Zitadel (Wildfly-based, ~1 GB memory at idle).
- ❌ Configuration surface is larger; more clicks to wire up federation.
- **Rejected because:** Zitadel offers the same self-hosting and federation story with a lighter footprint and an Organizations model that maps more directly to the tenant concept. Keycloak is a defensible alternative; Zitadel is a slightly better fit for this project's shape.

### IdP — Auth0

- ✅ Managed, polished UX, great docs.
- ✅ Federation works out of the box.
- ❌ Cloud-only — incompatible with [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)'s local-deployment driver. The API would have to reach Auth0 for every JWKS refresh.
- ❌ Per-MAU pricing past the free tier; locks the project into a SaaS bill if it grows.
- **Rejected because:** local-deployment compatibility is a hard driver here, and managed-IdP lock-in violates it.

### IdP — AWS Cognito

- ✅ Managed, integrates with AWS ecosystem.
- ❌ AWS-locked; an awkward dependency for a non-AWS-hosted project.
- ❌ Federation polish is weaker than Zitadel/Keycloak/Auth0.
- ❌ Cloud-only — same local-deployment incompatibility as Auth0.
- **Rejected because:** wrong cloud, and the project doesn't have an AWS dependency to leverage.

### IdP — Ory Hydra

- ✅ Standards-purist OAuth2/OIDC server; highly composable.
- ✅ Self-hostable.
- ❌ Headless — needs Ory Kratos for identity management and a custom UI for login. Operating two services where one would do.
- ❌ Federation support requires more wiring than Zitadel's UI-driven flow.
- **Rejected because:** the operational cost of running Hydra + Kratos + a custom login UI is too high for a single-maintainer personal project.

### Validation — local JWT (chosen)

- ✅ Fast (no per-request network hop).
- ✅ Tolerates brief IdP unavailability for already-issued tokens.
- ✅ Spring Security's resource-server is mature, well-documented, and one config block.
- ❌ No revocation for in-flight access tokens — mitigated by the 15-minute TTL.

### Validation — OAuth 2.0 Token Introspection (RFC 7662)

- ✅ Centralized revocation: the IdP can invalidate a token immediately.
- ✅ Useful when tokens are opaque (not JWTs).
- ❌ Every API call round-trips to the IdP — adds latency and creates a hard dependency on IdP availability.
- ❌ Operationally fragile: a momentary Zitadel hiccup turns into a 5xx storm on the API.
- **Rejected because:** the latency cost and the availability coupling are not worth a revocation window we can shrink to 15 minutes by other means.

## Confirmation

How we will know this decision is being followed:

- **Configuration scan**: `grep -RE 'oauth2\.resourceserver\.opaquetoken' src/main` returns zero matches. Introspection is forbidden.
- **Configuration scan**: `grep -RE 'oauth2\.resourceserver\.jwt\.(issuer-uri|jwk-set-uri)' src/main` returns at least one match in `application*.yml`. Local JWT validation is configured.
- **Algorithm allow-list**: the Spring Security `JwtDecoder` configuration explicitly allow-lists RS256/ES256. A unit test feeds an HS256-signed token and asserts the decoder rejects it.
- **Dependency-tree scan**: resolved `runtimeClasspath` contains no `com.google.api-client:google-api-client`, `com.google.oauth-client:*`, `com.google.auth:*`, MSAL (`com.microsoft.azure:msal4j`), Apple Sign-In SDK, or any other upstream-IdP client. Only Spring Security's OAuth2 resource-server and JWT support are present.
- **Architecture test (federation isolation)**: an ArchUnit / Konsist test asserts no class in `src/main/**` imports from `com.google.*` (auth-related packages), `com.microsoft.aad.*`, `com.apple.*`, or any other upstream-IdP namespace. The application sees Zitadel JWTs only.
- **Token-claims test**: an integration test signs a sample JWT with a test JWKS, confirms the resource-server decodes it, extracts `tid`, and populates `TenantContext`. Missing `tid` causes a 401 (or 403) — never a request that proceeds with `tid = null`.
- **Manual review**: any PR that adds an OAuth client library, modifies `spring.security.oauth2.*` configuration, or touches the JWT decoder's algorithm allow-list MUST be reviewed against this ADR and rejected unless the change aligns or the ADR is amended.

## More Information

- [ADR-002 — Multi-tenant isolation strategy](ADR-002-multi-tenant-isolation.md) — defines the `tid` claim contract.
- [ADR-004 — Two-store persistence — Axon Server for events, MongoDB for read models](ADR-004-two-store-persistence-axon-server-mongo.md) — local-deployment driver inherited here.
- `docs/guidelines/api-rules.md` — the API guideline that requires OAuth 2.0 Bearer authentication on every endpoint.
- Zitadel documentation: https://zitadel.com/docs
- OAuth 2.0 Security Best Current Practice (RFC 9700, formerly draft-ietf-oauth-security-topics)
- Spring Security OAuth2 Resource Server: https://docs.spring.io/spring-security/reference/servlet/oauth2/resource-server/jwt.html

<!-- Directives for edikt governance. Populated by /edikt:adr:compile. -->
[edikt:directives:start]: #
source_hash: 6c6c8a6907cd639b4453ec6400db1821767c661517b0a1adf68c389548d8407f
directives_hash: ef7b4ee59873d8c5e888abb4fe836f6e0e66fab10a9c8ed35774759f04f4f1d6
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/application*.yml"
  - "**/application*.yaml"
  - "gradle/libs.versions.toml"
scope:
  - planning
  - design
  - implementation
  - review
directives:
  - "The OAuth 2.0 / OIDC identity provider MUST be Zitadel. Production, staging, and local-development environments all authenticate against a Zitadel instance running as a container. NEVER integrate Keycloak, Auth0, AWS Cognito, Ory Hydra, or any other IdP into the Spring Boot application. (ref: ADR-005)"
  - "The API MUST validate JWTs locally using Spring Security's resource-server with `spring.security.oauth2.resourceserver.jwt.issuer-uri` (or `jwk-set-uri`) pointing at the Zitadel issuer. NEVER use OAuth 2.0 Token Introspection (RFC 7662) — `spring.security.oauth2.resourceserver.opaquetoken.*` properties MUST NOT appear in any `application*.yml`. (ref: ADR-005)"
  - "JWTs MUST be signed with RS256 or ES256 (asymmetric algorithms). NEVER configure, accept, or fall back to HS256 — symmetric signing requires sharing the signing key with every validator and is incompatible with the local-validation model. (ref: ADR-005)"
  - "Federation with external identity providers (Google, Apple, Microsoft, GitHub, generic OIDC) MUST be configured at the Zitadel side via Zitadel's external IDPs feature. NEVER integrate `google-oauth-client`, `google-api-services-*`, Apple Sign-In SDKs, MSAL (`com.microsoft.azure:msal4j`), or any other upstream-IdP client directly from the Spring Boot application — the application MUST only see Zitadel-issued JWTs. (ref: ADR-005)"
  - "Every access token MUST carry a `tid` claim populated from Zitadel's Organization ID (configured via a Zitadel Action or claim mapper) per ADR-002. The Spring Security `JwtAuthenticationConverter` MUST extract `tid` and populate the request-scoped `TenantContext`. (ref: ADR-005)"
  - "Access-token lifetime MUST be ≤ 15 minutes. NEVER configure access-token TTL longer than 15 minutes at Zitadel — local validation has no revocation channel, and longer lifetimes widen the post-revocation attack window unacceptably. (ref: ADR-005)"
  - "Refresh tokens MUST rotate on every use: each refresh exchange MUST issue a new refresh token and invalidate the prior one. NEVER configure non-rotating (long-lived static) refresh tokens at Zitadel. (ref: ADR-005)"
  - "Refresh tokens MUST have an absolute lifetime ≤ 30 days regardless of rotation, and an inactivity timeout of ≤ 14 days (a refresh token unused for 14 days is revoked). NEVER allow a session chain to extend indefinitely. (ref: ADR-005)"
  - "Spring Security's JWT decoder MUST cache Zitadel's JWKS and refresh on `kid` mismatch or after a TTL of ≤ 24 hours. NEVER hardcode public keys, NEVER bypass JWKS validation, and NEVER pin a single `kid` — key rotation at the IdP must Just Work. (ref: ADR-005)"
reminders:
  - "Before adding an upstream-IdP client library (Google OAuth, MSAL, Apple Sign-In, etc.) → don't; federation lives at Zitadel, the Spring Boot app sees only Zitadel JWTs (ref: ADR-005)"
  - "Before configuring Spring Security's JWT decoder or `application*.yml` OAuth2 properties → use `oauth2.resourceserver.jwt.issuer-uri` pointing at Zitadel; never `opaquetoken.*` (introspection), never hardcoded keys, never HS256 (ref: ADR-005)"
verification:
  - "[ ] `application*.yml` configures `spring.security.oauth2.resourceserver.jwt.issuer-uri` (or `jwk-set-uri`) at a Zitadel host; no `spring.security.oauth2.resourceserver.opaquetoken.*` keys are present (ref: ADR-005)"
  - "[ ] Spring Security's `JwtDecoder` is configured to allow only RS256 / ES256; a unit test feeds an HS256-signed token and asserts the decoder rejects it (ref: ADR-005)"
  - "[ ] Resolved `runtimeClasspath` contains no `com.google.api-client:*`, `com.google.oauth-client:*`, `com.google.auth:*`, `com.microsoft.azure:msal4j`, Apple Sign-In SDKs, or any other upstream-IdP client library (ref: ADR-005)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
