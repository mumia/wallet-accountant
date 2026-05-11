# API Rules Guidelines

**Purpose:** Lock the API surface into an API-first contract — every endpoint exists in OpenAPI 3.1 before it exists in code, errors share a single RFC 7807 envelope, naming and pagination are uniform, and authentication is required by default — so that consumers see one API, not a patchwork of per-team conventions.

## Rationale

APIs that ship without a contract drift in three directions at once: schema drift between client expectations and server reality, error-format drift between endpoints, and naming drift across resources. The fix isn't documentation written *after* the fact; it's API-first design, where the OpenAPI 3.1 specification is the planning artifact and the controllers are derived from it. Once the contract exists, the rest follows: Zalando's [RESTful API Guidelines](https://opensource.zalando.com/restful-api-guidelines) as the canonical reference for HTTP semantics and resource modeling, RFC 7807 `application/problem+json` as the single error envelope, cursor-based pagination with `items` + `next` so consumers never have to special-case offset arithmetic, and a small set of naming rules (`kebab-case` paths, `snake_case` query parameters, `SCREAMING_SNAKE_CASE` enums) that make every endpoint feel like it came from the same team.

Swagger UI is the runtime mirror of the OpenAPI contract — it closes the gap between "the spec we shipped" and "the spec the running service exposes". Bearer-token (OAuth 2.0) is the default authentication mode; public endpoints are explicit, not implicit. Trailing-slash requests are rejected (not 301-redirected) so clients fail loud rather than rely on a gateway-specific normalization they may not have. These rules add up to one principle: API consumers should be able to integrate against the OpenAPI document alone and never be surprised by the running service.

## Rules

- An OpenAPI 3.1 specification MUST be authored, reviewed, and committed during the planning phase for every feature that introduces or modifies API endpoints. NEVER write controller code, request/response DTOs, or contract tests for an endpoint before its OpenAPI 3.1 contract is committed.
- The OpenAPI 3.1 contract MUST be exposed at runtime via Swagger UI (`/swagger-ui`) and the raw spec endpoint (`/v3/api-docs`). NEVER deploy a service whose Swagger UI returns a different schema than the committed `openapi.yaml`.
- API designs MUST conform to the Zalando RESTful API Guidelines (https://opensource.zalando.com/restful-api-guidelines). NEVER ship an endpoint that violates any "MUST" rule from those guidelines without an accompanying ADR documenting and justifying the deviation.
- List endpoints MUST use cursor-based pagination with `cursor` and `limit` query parameters; the response body MUST contain an `items` array and a `next` cursor field. NEVER use offset / page-number pagination, and NEVER return paginated results without a `next` field (use `null` to signal the final page).
- Every error response (4xx and 5xx) MUST use the `application/problem+json` media type and conform to RFC 7807, including at minimum the `type`, `title`, and `status` fields. Custom extension members are permitted, but the base structure MUST NOT deviate from RFC 7807. NEVER return error bodies as plain `application/json` for any 4xx or 5xx response.
- URL path segments MUST be `kebab-case` lowercase ASCII (e.g., `/account-balances`, `/transaction-categories`). NEVER use `camelCase`, `snake_case`, or mixed-case in URL path segments.
- Query parameter keys MUST be `snake_case` lowercase ASCII (e.g., `?account_id=…&include_deleted=false`). NEVER use `camelCase`, `kebab-case`, or `SCREAMING_SNAKE_CASE` in query parameter keys.
- Every endpoint MUST require Bearer-token (OAuth 2.0) authentication unless it is explicitly declared public in the OpenAPI `security` scheme with documented justification. NEVER accept tokens transmitted in query parameters or request bodies, and NEVER expose an authenticated resource through an unauthenticated alias.
- Requests with a trailing slash on the path MUST be rejected with `404 Not Found` and an `application/problem+json` body. NEVER 301 / 307-redirect trailing-slash requests, and NEVER silently strip the slash.
- Enumerated string values in requests, responses, and OpenAPI schemas MUST be `SCREAMING_SNAKE_CASE` (e.g., `STATUS_PENDING`, `ACCOUNT_TYPE_SAVINGS`, `CURRENCY_EUR`). NEVER use lowercase, `camelCase`, or `kebab-case` for enum string values.

## When NOT to apply

These rules do not apply to:

- **Internal management endpoints** under Spring Actuator (`/actuator/**`) and any operator-only debug endpoints exposed on a private management port. They are not part of the public API contract and do not need to live in the OpenAPI 3.1 spec, follow the kebab-case path rule, or return `application/problem+json` errors. They MUST still require authentication if reachable from any non-private network.
- **Webhook receivers consuming third-party payloads** that arrive with their own schema (e.g., Stripe webhooks, OAuth provider callbacks). The receiver endpoint MUST follow the path / query / authentication rules of this guideline at the *boundary*, but the request body SHOULD match the third party's published contract verbatim — translation to the project's domain model happens after parsing.
- **Generated code** emitted by an OpenAPI codegen tool. If the generator violates a naming rule (e.g., turns `snake_case` query keys into `camelCase` Kotlin properties), the fix is in the generator configuration or the spec, not in hand-edits to generated files.
- **Swagger UI in production deployments** MAY be gated behind authentication, an internal listener, or an IP allowlist for security reasons. The "MUST expose Swagger UI" rule still holds in dev / staging; in prod the *exposure* may be restricted, but the *Swagger UI must still serve the same OpenAPI document* the service ships with.

These four exceptions are the only legitimate ones. "We'll write the spec after the controller is done", "errors are easier to debug as plain JSON", and "the team uses camelCase in query params on this one endpoint" are not exceptions — they are exactly the failure modes this guideline exists to prevent.

---

*Created by edikt:guideline — 2026-05-01*

<!-- Directives for edikt governance. Populated by /edikt:guideline:compile. -->
[edikt:directives:start]: #
source_hash: 78d980433d9f93e9520b22a8828d9b11eee6f8b4474b5e64eedd456108848227
directives_hash: ba90449ec6c35e265f067161724315b7fd1aa93d4be35d37078451d1ce6f748c
compiler_version: "0.4.3"
paths:
  - "**/adapter/in/web/**"
  - "**/*.kt"
  - "**/openapi*.yaml"
  - "**/openapi*.yml"
  - "**/*-api.yaml"
  - "**/*-api.yml"
scope:
  - design
  - implementation
  - review
directives:
  - "An OpenAPI 3.1 specification MUST be authored, reviewed, and committed during the planning phase for every feature that introduces or modifies API endpoints. NEVER write controller code, request/response DTOs, or contract tests for an endpoint before its OpenAPI 3.1 contract is committed. (ref: api-rules)"
  - "The OpenAPI 3.1 contract MUST be exposed at runtime via Swagger UI (`/swagger-ui`) and the raw spec endpoint (`/v3/api-docs`). NEVER deploy a service whose Swagger UI returns a different schema than the committed `openapi.yaml`. (ref: api-rules)"
  - "API designs MUST conform to the Zalando RESTful API Guidelines (https://opensource.zalando.com/restful-api-guidelines). NEVER ship an endpoint that violates any \"MUST\" rule from those guidelines without an accompanying ADR documenting and justifying the deviation. (ref: api-rules)"
  - "List endpoints MUST use cursor-based pagination with `cursor` and `limit` query parameters; the response body MUST contain an `items` array and a `next` cursor field. NEVER use offset / page-number pagination, and NEVER return paginated results without a `next` field (use `null` to signal the final page). (ref: api-rules)"
  - "Every error response (4xx and 5xx) MUST use the `application/problem+json` media type and conform to RFC 7807, including at minimum the `type`, `title`, and `status` fields. Custom extension members are permitted, but the base structure MUST NOT deviate from RFC 7807. NEVER return error bodies as plain `application/json` for any 4xx or 5xx response. (ref: api-rules)"
  - "URL path segments MUST be `kebab-case` lowercase ASCII (e.g., `/account-balances`, `/transaction-categories`). NEVER use `camelCase`, `snake_case`, or mixed-case in URL path segments. (ref: api-rules)"
  - "Query parameter keys MUST be `snake_case` lowercase ASCII (e.g., `?account_id=…&include_deleted=false`). NEVER use `camelCase`, `kebab-case`, or `SCREAMING_SNAKE_CASE` in query parameter keys. (ref: api-rules)"
  - "Every endpoint MUST require Bearer-token (OAuth 2.0) authentication unless it is explicitly declared public in the OpenAPI `security` scheme with documented justification. NEVER accept tokens transmitted in query parameters or request bodies, and NEVER expose an authenticated resource through an unauthenticated alias. (ref: api-rules)"
  - "Requests with a trailing slash on the path MUST be rejected with `404 Not Found` and an `application/problem+json` body. NEVER 301 / 307-redirect trailing-slash requests, and NEVER silently strip the slash. (ref: api-rules)"
  - "Enumerated string values in requests, responses, and OpenAPI schemas MUST be `SCREAMING_SNAKE_CASE` (e.g., `STATUS_PENDING`, `ACCOUNT_TYPE_SAVINGS`, `CURRENCY_EUR`). NEVER use lowercase, `camelCase`, or `kebab-case` for enum string values. (ref: api-rules)"
reminders:
  - "Before adding or modifying an API endpoint → write the OpenAPI 3.1 contract first; controllers and DTOs come only after the spec is committed (ref: api-rules)"
  - "Before returning a 4xx/5xx error → use `application/problem+json` with at minimum `type`, `title`, and `status` per RFC 7807 (ref: api-rules)"
verification:
  - "[ ] Every controller endpoint under `adapter/in/web/**` has a matching path/method entry in the committed OpenAPI 3.1 spec (ref: api-rules)"
  - "[ ] All URL path segments are `kebab-case`, all query parameter keys are `snake_case`, all enum string values are `SCREAMING_SNAKE_CASE` (ref: api-rules)"
  - "[ ] All 4xx/5xx responses set `Content-Type: application/problem+json` and include the `type`, `title`, and `status` fields (ref: api-rules)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
