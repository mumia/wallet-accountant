---
status: accepted
date: 2026-05-10
decision-makers: [Miguel Manso]
consulted: []
informed: []
supersedes: null
---

# ADR-006: Zalando RESTful API Guidelines + OpenAPI 3.1 + RFC 7807 as the API contract foundation

## Context and Problem Statement

wallet-accountant exposes an HTTP+JSON API through `adapter/in/web/**`. `docs/guidelines/api-rules.md` already encodes the implementation rules (kebab-case paths, snake_case query params, SCREAMING_SNAKE_CASE enums, cursor pagination, `application/problem+json` errors, OpenAPI 3.1 spec-first, Bearer auth). What that guideline does *not* explicitly record is the meta-choice it implements: **which industry style guide is the project bound to?** A future PR could plausibly amend api-rules to drift toward a different convention (Google AIP's `:custom_method` verbs, JSON:API's envelope shape, Microsoft's `@nextLink` pagination) without anyone noticing the foundational change.

Three style-guide families are mature and worth weighing:

1. **Zalando RESTful API Guidelines** — public, comprehensive, ~200 numbered rules covering naming, evolution, errors, pagination, versioning, idempotency. CC BY 4.0, actively maintained.
2. **Google AIP (API Improvement Proposals)** — public, resource-oriented, optimized for gRPC + REST transcoding. Strong on long-running operations, field masks, and resource hierarchies.
3. **Microsoft REST API Guidelines** — public, pragmatic, Azure-flavored. Covers async patterns, versioning, error format. Less comprehensive than Zalando on naming.

Plus two narrower comparators:

4. **JSON:API** — a strict media-type specification. Single envelope shape (`{ data, included, links, relationships }`), single error format. Highly prescriptive.
5. **Heroku HTTP API Design Guide** / **roll your own** — minimal style guides or in-house conventions.

Error-response format is its own decision axis with two main contenders: **RFC 7807 Problem Details** (`application/problem+json`) — IETF standard, used by Zalando — and **JSON:API errors** (`{ "errors": [...] }` envelope) — bundled into JSON:API, incompatible with RFC 7807 on the wire.

OpenAPI version is yet another axis: 3.1 (current, JSON-Schema-aligned), 3.0 (older, JSON-Schema-2012 mismatch), Swagger 2.0 (legacy), RAML / API Blueprint (off-trend).

How should we choose the API style guide, the OpenAPI version, and the error-format spec — and lock them in so the implementation guideline can't silently drift?

## Decision Drivers

- **Comprehensiveness.** A small project with one maintainer can't write its own 200-rule style guide; the chosen foundation must cover the long tail (versioning, idempotency, evolution, pagination shape, header conventions).
- **HTTP-native, not gRPC-flavored.** wallet-accountant is plain HTTP+JSON. The style guide should have been written for that, not for gRPC transcoded to REST.
- **Public, externally maintained, freely licensed.** No vendor lock-in, no per-decision argument from first principles.
- **Composes with [ADR-005](ADR-005-oauth-zitadel-local-jwt.md)'s OAuth 2.0 Bearer model and [ADR-002](ADR-002-multi-tenant-isolation.md)'s `tid` claim** — the chosen style guide must not require a different auth scheme or a tenant-routing convention (e.g., subdomain-per-tenant) that contradicts decisions already made.
- **Single error format across the whole API.** Whatever we pick must rule out per-endpoint or per-error-type ad-hoc shapes.
- **Codify the meta-choice once, so the implementation guideline can be audited against a fixed reference** — future api-rules amendments must trace back to a Zalando rule (or an explicit, documented deviation from one), not to opinion.

## Considered Options

1. **Zalando RESTful API Guidelines + OpenAPI 3.1 + RFC 7807** — public 200+ numbered rules, HTTP-native, RFC 7807 errors, OpenAPI-first.
2. **Google AIP + OpenAPI 3.1 + Google's `google.rpc.Status` error format** — resource-oriented, gRPC-flavored, REST is a transcoding output.
3. **Microsoft REST API Guidelines + OpenAPI 3.1 + Microsoft's `OData`/`InnerError` format** — pragmatic, Azure-flavored.
4. **JSON:API** — fixed media type, fixed envelope, fixed error format. Adopt the whole spec.
5. **Heroku HTTP API Design Guide / roll our own** — minimal published guide or in-house conventions documented in `docs/guidelines/api-rules.md` alone.

## Decision Outcome

Chosen combination: **Zalando RESTful API Guidelines + OpenAPI 3.1 + RFC 7807**, because it gives us the broadest coverage of HTTP+JSON conventions with the least friction against decisions already made (OAuth Bearer, tenant claim, hexagonal layout), and it pins all three sub-axes (style guide, OpenAPI version, error format) to publicly maintained, freely licensed standards.

The decision lands as the following hard rules:

- The authoritative API style guide MUST be the **Zalando RESTful API Guidelines** (https://opensource.zalando.com/restful-api-guidelines/). `docs/guidelines/api-rules.md` is the project-local implementation of that guide; every rule in it MUST trace back to a Zalando rule or document an explicit deviation. NEVER adopt Google AIP, Microsoft REST API Guidelines, JSON:API, or any other top-level style guide as a competing source — when conventions diverge between guides, Zalando wins.
- The OpenAPI specification version MUST be **3.1.x**. The committed spec file's `openapi:` field MUST be `3.1.0` or higher. NEVER write or accept OpenAPI 3.0.x (JSON Schema 2012-12 mismatch), Swagger 2.0, RAML, or API Blueprint as the API contract.
- HTTP error responses (4xx and 5xx) MUST use the **RFC 7807 Problem Details for HTTP APIs** structure with the `application/problem+json` media type. The body MUST contain at minimum `type`, `title`, and `status`; `detail` and `instance` are recommended; extension members are allowed and SHOULD be `kebab-case`. NEVER use JSON:API error envelopes (`{ "errors": [...] }`), custom error wrappers (`{ "code": "…", "message": "…" }` outside Problem Details), `text/plain` error bodies, or HTML error pages from a JSON endpoint.
- The OpenAPI document MUST be the **source of truth** for the API contract — written before the controller, committed to the repository, and rendered in the running service via Swagger UI (or equivalent). NEVER allow controller code to ship without a matching path/method entry in the committed OpenAPI document. `docs/guidelines/api-rules.md` carries the per-endpoint enforcement; this directive freezes the foundational position.
- Where the api-rules guideline diverges from a Zalando rule, the divergence MUST be explicitly documented in api-rules with the Zalando rule number and a one-line reason. NEVER silently override a Zalando convention; either follow it, or call out the deviation.

### Consequences

**Positive:**
- A single comprehensive reference. Every API-design question (versioning, idempotency, pagination, headers, evolution rules) has a Zalando rule to consult — no first-principles re-derivation.
- Composability with prior decisions: Zalando uses Bearer auth (matches [ADR-005](ADR-005-oauth-zitadel-local-jwt.md)), is tenancy-agnostic at the URL layer (matches [ADR-002](ADR-002-multi-tenant-isolation.md)'s JWT-claim approach), and pins OpenAPI as the contract first (matches the existing api-rules guideline).
- Single wire-error format: every 4xx/5xx body looks the same. Clients write one error parser.
- The api-rules guideline becomes auditable against a fixed external reference. A future amendment that adds, say, `:reset` verb suffixes (a Google AIP idiom) can be flagged by reviewers as a deviation from Zalando.
- OpenAPI 3.1 unlocks current JSON Schema (2020-12), which lets us reuse domain JSON schemas without the 3.0 dialect mismatch.

**Negative:**
- Some Zalando-specific assumptions (e.g., `X-Flow-Id` for cross-service tracing, partner ecosystem language) don't perfectly fit a single-maintainer personal project. The api-rules guideline can document those as "applied" or "deviated" per-rule.
- Tooling around OpenAPI 3.1 is younger than 3.0; some code-generators and renderers lag. Swagger UI 5.x supports 3.1 natively; older client generators may not. Mitigated by sticking to widely supported features.
- Error-format lock-in: introducing a third-party endpoint that emits non-Problem-Details errors (e.g., a webhook with a vendor-specific error body) requires an adapter at the boundary, not a project-wide format relaxation.

**Neutral:**
- The Zalando guide is long. Engineers don't need to memorise it; the api-rules guideline is the day-to-day reference, with Zalando as the authoritative tiebreaker.
- This ADR does not redo the per-endpoint enforcement that already lives in api-rules — it pins the *foundation* the guideline implements.

## Pros and Cons of the Options

### 1. Zalando + OpenAPI 3.1 + RFC 7807 (chosen)

- ✅ Most comprehensive published HTTP+JSON style guide; ~200 numbered rules covering the long tail.
- ✅ Public, CC BY 4.0, actively maintained on GitHub.
- ✅ HTTP-native (not gRPC-derived). Direct fit for plain REST.
- ✅ Mandates RFC 7807 — IETF-standard error format, single shape across the API.
- ✅ Mandates spec-first OpenAPI — the api-rules guideline already enforces this.
- ❌ A handful of Zalando-specific conventions (`X-Flow-Id`, partner-API language) don't apply directly; need documented "applied" / "deviated" stance per such rule.

### 2. Google AIP + OpenAPI 3.1 + `google.rpc.Status`

- ✅ Excellent on long-running operations, field masks, hierarchical resources.
- ✅ Used by Google's public APIs at scale.
- ❌ Designed for gRPC + REST transcoding. Conventions like `:custom_method` URL verbs and `?fields=` field masks feel awkward in a pure HTTP+JSON setup.
- ❌ The error format (`google.rpc.Status` with `code`/`message`/`details`) competes with RFC 7807 — adopting AIP weakens the IETF-standard error story.
- ❌ AIPs evolve and occasionally conflict; cherry-picking sub-AIPs leads to inconsistency.
- **Rejected because:** wallet-accountant is plain HTTP+JSON, not gRPC. AIP's strengths address problems we don't have, and its error format would force us off RFC 7807.

### 3. Microsoft REST API Guidelines + OpenAPI 3.1 + Microsoft's error shape

- ✅ Pragmatic, well-documented, used by Microsoft Graph and Azure.
- ✅ Covers async patterns and versioning.
- ❌ Less comprehensive than Zalando on naming, evolution, and pagination (Microsoft has `@nextLink`, Zalando has cursor pagination — both work, but Zalando documents the trade-offs more thoroughly).
- ❌ Azure / Office 365 assumptions in places; somewhat ecosystem-flavored.
- ❌ Error-shape recommendations (`InnerError` chain) duplicate work that RFC 7807 already does cleanly.
- **Rejected because:** Zalando covers the same ground more comprehensively without the Azure flavor; Microsoft guidelines are a defensible alternative, but a slightly weaker fit for a non-Microsoft-ecosystem project.

### 4. JSON:API

- ✅ Fixed media type, fixed envelope — zero ambiguity.
- ✅ Single error format (incompatible with 7807 but internally consistent).
- ❌ The `data`/`included`/`links`/`relationships` envelope is verbose for simple CRUD. A "get an account" response is twice the bytes of a plain JSON resource.
- ❌ Adopting JSON:API means rejecting RFC 7807 errors — a meaningful loss of an IETF-standard format.
- ❌ JSON:API does not cover versioning, evolution, header conventions, or pagination strategies in the depth Zalando does. We'd still need a higher-level style guide on top.
- **Rejected because:** the envelope tax is high for the project's actual API shape, and the error-format incompatibility with RFC 7807 is a regression.

### 5. Heroku HTTP API Design Guide / roll our own

- ✅ Heroku's guide is short and opinionated; easy to adopt fully.
- ✅ Rolling our own gives total control.
- ❌ Heroku covers a fraction of what Zalando does. We'd hit gaps within weeks (versioning strategy, idempotency keys, pagination shape).
- ❌ Rolling our own is reinventing the wheel — every rule is an argument from first principles, every consumer has to learn idiosyncratic conventions.
- **Rejected because:** the maintenance cost of a hand-rolled style guide on a single-maintainer project is not justified when Zalando is free, comprehensive, and externally maintained.

## Confirmation

How we will know this decision is being followed:

- **Spec version check**: a CI step asserts the committed OpenAPI document's top-level `openapi:` field starts with `3.1`. Any other version (`3.0.x`, `2.0`) fails the build.
- **Error-format scan**: a CI step (`grep -RE '"errors"\s*:\s*\[' src/main`) returns zero matches in production code. JSON:API error envelopes are forbidden. A second scan asserts the `@RestControllerAdvice` returns `ProblemDetail` (or equivalent RFC 7807 type), never a custom error wrapper.
- **OpenAPI lint**: an OpenAPI linter (Spectral with a Zalando-flavored ruleset, or `zalando-rest-api-guidelines` Spectral rules) runs against the committed spec on every PR. Lint failures block merge.
- **Style-guide deviation registry**: any rule in `docs/guidelines/api-rules.md` that diverges from a Zalando rule MUST cite the Zalando rule number (e.g., "Deviates from MUST 174: …") with a one-line reason. A reviewer can grep for "Deviates from" to audit the deviation surface; absence of such markers means the guideline matches Zalando rule-for-rule.
- **Manual review**: any PR that adds a new style or error convention to `docs/guidelines/api-rules.md` MUST be reviewed against this ADR. New conventions either trace to a Zalando rule, document an explicit deviation, or fail review.

## More Information

- [ADR-002 — Multi-tenant isolation strategy](ADR-002-multi-tenant-isolation.md) — JWT `tid` claim (Zalando-compatible — claim-based, not URL-based, tenancy).
- [ADR-005 — OAuth 2.0 / OIDC: Zitadel as IdP, local JWT validation, Google federation](ADR-005-oauth-zitadel-local-jwt.md) — Bearer-token auth model that this style guide assumes.
- `docs/guidelines/api-rules.md` — the project-local implementation of the rules locked here.
- Zalando RESTful API Guidelines: https://opensource.zalando.com/restful-api-guidelines/
- OpenAPI 3.1.0 specification: https://spec.openapis.org/oas/v3.1.0
- RFC 7807 — Problem Details for HTTP APIs: https://datatracker.ietf.org/doc/html/rfc7807
- Spectral linter: https://stoplight.io/open-source/spectral

<!-- Directives for edikt governance. Populated by /edikt:adr:compile. -->
[edikt:directives:start]: #
source_hash: 8c48cdf2d04aa0a4a227d72a6a28c05cacbd37df5c8ca4e437e20839b62e30e3
directives_hash: 365296a6ac1d83cc91493b0b20740b5a1adac2e36bc98a9a241c8b046115d143
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/openapi*.yml"
  - "**/openapi*.yaml"
  - "**/openapi*.json"
  - "**/api/**"
  - "docs/guidelines/api-rules.md"
scope:
  - planning
  - design
  - implementation
  - review
directives:
  - "The authoritative API style guide MUST be the Zalando RESTful API Guidelines (https://opensource.zalando.com/restful-api-guidelines/). `docs/guidelines/api-rules.md` is the project-local implementation of that guide; every rule in it MUST trace back to a Zalando rule or document an explicit deviation. NEVER adopt Google AIP, Microsoft REST API Guidelines, JSON:API, or any other top-level style guide as a competing source — when conventions diverge between guides, Zalando wins. (ref: ADR-006)"
  - "The OpenAPI specification version MUST be 3.1.x. The committed spec file's top-level `openapi:` field MUST start with `3.1`. NEVER write or accept OpenAPI 3.0.x, Swagger 2.0, RAML, or API Blueprint as the API contract. (ref: ADR-006)"
  - "HTTP error responses (4xx and 5xx) MUST use the RFC 7807 Problem Details for HTTP APIs structure with the `application/problem+json` media type. The body MUST contain at minimum `type`, `title`, and `status`; `detail` and `instance` are recommended; extension members are allowed and SHOULD be `kebab-case`. NEVER use JSON:API error envelopes (`{ \"errors\": [...] }`), custom error wrappers (`{ \"code\": \"…\", \"message\": \"…\" }` outside Problem Details), `text/plain` error bodies, or HTML error pages from a JSON endpoint. (ref: ADR-006)"
  - "The OpenAPI document MUST be the source of truth for the API contract — written before the controller, committed to the repository, and rendered in the running service via Swagger UI (or equivalent). NEVER allow controller code to ship without a matching path/method entry in the committed OpenAPI document. (ref: ADR-006)"
  - "Where the api-rules guideline diverges from a Zalando rule, the divergence MUST be explicitly documented in `docs/guidelines/api-rules.md` with the Zalando rule number and a one-line reason (format: `Deviates from MUST <NNN>: <reason>`). NEVER silently override a Zalando convention; either follow it, or call out the deviation with the rule reference. (ref: ADR-006)"
reminders:
  - "Before adopting a new API convention not already in `docs/guidelines/api-rules.md` → check the Zalando RESTful API Guidelines first; never reach for Google AIP, Microsoft REST, or JSON:API patterns just because they're documented elsewhere (ref: ADR-006)"
  - "Before designing an HTTP error response → use RFC 7807 `application/problem+json` with `type`, `title`, `status`; never JSON:API error envelopes, custom error wrappers, or plain-text error bodies (ref: ADR-006)"
verification:
  - "[ ] The committed OpenAPI document declares `openapi: 3.1.x` (no `3.0.x`, no Swagger 2.0, no RAML / API Blueprint) (ref: ADR-006)"
  - "[ ] All 4xx/5xx response definitions in the OpenAPI document use the `application/problem+json` content type and conform to RFC 7807 (`type`, `title`, `status` minimum); no JSON:API error envelopes (`{\"errors\": [...]}`) or custom error wrappers anywhere in the spec or in `@RestControllerAdvice` code (ref: ADR-006)"
  - "[ ] Every deviation in `docs/guidelines/api-rules.md` from a Zalando rule cites the Zalando rule number with the format `Deviates from MUST <NNN>: <reason>`; absence of such markers means the guideline matches Zalando rule-for-rule (ref: ADR-006)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
