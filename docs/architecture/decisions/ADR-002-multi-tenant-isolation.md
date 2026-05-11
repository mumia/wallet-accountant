---
status: accepted
date: 2026-05-04
decision-makers: [Miguel Manso]
consulted: []
informed: []
supersedes: null
---

# ADR-002: Multi-tenant isolation strategy

## Context and Problem Statement

wallet-accountant is multi-tenant: each tenant is an end user managing their own personal accounts. Tenant isolation is a hard architectural constraint, declared in `docs/project-context.md`:

> Every query against the read models in MongoDB (must be tenant-scoped). Every command against an aggregate (must verify tenant ownership before applying state changes). Every Restate workflow (the workflow context must carry the tenant identity end-to-end).

That constraint must be implemented along three orthogonal axes, each with its own design space:

1. **Write side.** Where does the tenant boundary sit in Axon Framework / Axon Server? One context per tenant? `tenantId` in every command and event payload? One event stream per tenant?
2. **Read side.** How are MongoDB read models segregated? Database-per-tenant? Collection-per-tenant? Row-level filtering by `tenantId`?
3. **API surface.** How is the tenant identified on inbound requests? Subdomain routing? `X-Tenant-Id` header? A JWT claim?

The decisions interlock — write-side isolation strength bounds the value of read-side isolation, and the API surface is the entry point that resolves the tenant before either is consulted. They must be decided together.

The expected scale is **fewer than 10 tenants** (personal project). That number changes the answer materially: at 1k+ tenants the operational cost of per-tenant Axon contexts and per-tenant Mongo databases dominates everything else; at <10 tenants both options are technically viable, and the decisive factors become operational simplicity and architectural symmetry.

How should we isolate tenant data and identity across the write side, the read side, and the API surface?

## Decision Drivers

- **Hard guarantees against cross-tenant data leakage** — the project's core promise to its end users.
- **Operational simplicity for a single-maintainer personal project** — minimize moving parts, run on a small footprint.
- **Architectural symmetry across the three axes** — the same isolation mental model on the write side, the read side, and the API surface, so reviewers always know what to look for.
- **Compatibility with [ADR-001](ADR-001-axon-5-restate-division-of-labor.md)** — Restate workflows must be able to propagate tenant identity through the orchestration boundary.
- **Compatibility with [INV-003](../invariants/INV-003-axon-server-sole-event-store.md)** — Axon Server remains the single event store; tenancy must not require a parallel persistence path.

## Considered Options

The decision is the **combination** of one option per axis. Listed per axis:

**Write side**
1. Single Axon Server context, `tenantId` on every command/event/query.
2. One Axon Server context per tenant, `tenantId` on every command/event as well (belt-and-braces).
3. One Axon Server context per tenant, no `tenantId` in payloads (context provides isolation; payload doesn't need to).

**Read side**
1. Single MongoDB database, every `@Document` carries `tenantId`, every read filters on `tenantId` (row-level filter).
2. Database-per-tenant — one MongoDB database per tenant; routing layer selects the database from the resolved `tenantId`.
3. Collection-per-tenant — one shared database, suffixed collections per tenant (`accounts_{tenantId}`).

**API surface**
1. Tenant identity carried in a signed JWT claim (`tid`), extracted at the `adapter/in/web/**` boundary.
2. Subdomain-based routing (`{tenantId}.wallet-accountant.example`) — DNS / reverse proxy resolves the tenant.
3. `X-Tenant-Id` HTTP header on every request — resolved by a Spring filter.

## Decision Outcome

**Chosen combination — write 1, read 1, API 1:**

- **Write side: single Axon Server context. `tenantId` on every command, query, and event.** All tenants share one Axon Server context. Every command, query, and domain event MUST carry a non-null `tenantId` of type `TenantId` (a domain value object). Aggregate state MUST persist its owning `tenantId` and aggregate `@CommandHandler` methods MUST validate that the incoming command's `tenantId` matches the aggregate's stored `tenantId` *before* emitting any event via `AggregateLifecycle.apply(...)`. A mismatch MUST raise a domain exception that prevents the state transition.
- **Read side: row-level filter.** One shared MongoDB database. Every `@Document` for tenant-scoped read models MUST contain a `tenantId` field. Every read-repository method MUST accept `tenantId` as the first parameter and MUST apply it as a filter. Every compound index on tenant-scoped collections MUST lead with `tenantId`. `@QueryHandler` methods that read tenant-scoped data MUST extract `tenantId` from the query message and propagate it to every read-model lookup.
- **API surface: JWT claim.** Inbound HTTP requests MUST carry a signed JWT containing a `tid` claim (a UUID string). A Spring filter / `OncePerRequestFilter` at the `adapter/in/web/**` boundary MUST verify the JWT signature, extract the `tid` claim, and place the resolved `TenantId` into a request-scoped tenant context. Controllers MUST read the tenant from this context — never from request bodies, query parameters, URL paths, or unauthenticated headers.
- **Restate boundary (per [ADR-001](ADR-001-axon-5-restate-division-of-labor.md)).** Restate workflows under `adapter/in/restate/**` MUST receive `tenantId` as part of their input and MUST propagate it on every Axon command/query they dispatch. NEVER dispatch a tenant-scoped Axon command/query from a Restate handler without an explicit `tenantId`.

The chosen combination optimizes for a single-maintainer project with <10 tenants where operational simplicity dominates, while keeping enforcement strong enough that cross-tenant leakage is prevented at every layer through static and runtime checks.

### Consequences

**Positive:**
- Single Axon context — one event log to operate, one set of tracking processors, one set of dashboards.
- Single MongoDB — one set of indexes, one migration path, one connection pool.
- The tenant model is *the same* on every axis: a `tenantId` value carried in the payload / document / claim. No mental model switch when crossing layers.
- `tenantId` lives in the event log forever — natural audit trail for "who did what" across the system's history.
- Easy to add tenants — no DB provisioning, no Axon-context creation, just an entry in the identity provider.
- Cross-tenant analytics (admin reporting, cohort metrics) is just a `$group` away if ever needed.

**Negative:**
- Soft isolation on the read side: a missed `tenantId` filter leaks data. The application layer is the only safety net. Mitigated by mandatory parameter shape, ArchUnit/Konsist tests, and integration tests that run two tenants and assert isolation — but the residual risk is non-zero.
- Soft isolation at the aggregate level: a forged or omitted `tenantId` in a command must be caught by the `@CommandHandler` validation. A bug in that validation is a tenancy bug.
- Per-tenant data export / deletion (e.g., GDPR right to erasure) requires query-and-delete across collections, not a clean `dropDatabase()`.
- A noisy-neighbor tenant shares index/cache pressure with other tenants — acceptable at <10 tenants.

**Neutral:**
- All commands, queries, and events gain a `tenantId` field. This is a one-time schema commitment; the field never goes away.
- Read-repository signatures are uniform: every tenant-scoped read takes `tenantId` first.
- An `@AuthenticationPrincipal` / request-scoped `TenantContext` bean becomes part of the standard Spring filter chain.

## Pros and Cons of the Options

### Write side

#### 1. Single context + tenantId in payloads (chosen)
- ✅ One operational unit (one event log, one tracking processor pool, one dashboard).
- ✅ Tenant attribution is in every event by construction — the audit trail is automatic.
- ✅ Adding a tenant is a no-op for the event store.
- ❌ Soft isolation: `@CommandHandler` validation is the only fence. A bug there is a tenancy bug.

#### 2. Context-per-tenant + tenantId in payloads (rejected — belt-and-braces overkill at this scale)
- ✅ Hard physical isolation of the event log per tenant; cross-tenant leakage in the write side is impossible.
- ✅ Per-tenant offboarding is a clean "drop the context" operation.
- ❌ Axon Server contexts are operational units (own oplog, own cluster registration, own ACLs). Even at <10 tenants, this multiplies operations work for a personal project.
- ❌ Asymmetric with the chosen read-side option (single shared Mongo) — payload-based isolation on read is the load-bearing rule anyway, so the write-side hard isolation buys less than it costs.
- Rejected because: at <10 tenants the operational tax of N contexts is not justified by the marginal isolation gain over `@CommandHandler` validation.

#### 3. Context-per-tenant, no tenantId in payloads (rejected — context-only)
- ✅ Single isolation mechanism — context.
- ❌ Loses cross-cutting attribution: events have no tenantId field, so any observer outside the context (analytics, archival) cannot tell which tenant produced an event without reading the context metadata.
- ❌ Restate workflows that span the Axon boundary need *some* `tenantId` in the message anyway — re-introducing what option 3 removed.
- Rejected because: dropping `tenantId` from payloads creates exactly the gap that `tenantId`-in-payload was designed to close.

### Read side

#### 1. Row-level filter (chosen)
- ✅ One database, one set of indexes, one connection pool.
- ✅ Architectural symmetry with the chosen write side (single shared store, `tenantId` is the isolation key).
- ✅ Cheapest hosting, simplest migrations.
- ❌ Soft isolation. Mitigated by repository discipline + arch tests + integration tests, but never zero risk.

#### 2. Database-per-tenant (rejected — operationally heavier without proportional benefit at this scale)
- ✅ Hard physical isolation. Per-tenant `mongodump --db` and `dropDatabase()`.
- ✅ Per-tenant performance isolation; one heavy tenant doesn't share index/cache.
- ❌ N times the migrations, N connection routings, N times the schema-evolution work — even N=10 means 10 of everything to keep in lockstep.
- ❌ Asymmetric with the chosen write side (single Axon context).
- Rejected because: at <10 tenants the operational symmetry with the single-context write side is more valuable than the hard read-side isolation; the soft read-side isolation is enforceable through arch tests and repository signatures.

#### 3. Collection-per-tenant (rejected — anti-pattern in MongoDB)
- ❌ Collection-explosion in MongoDB (each collection has its own indexes, cache footprint).
- ❌ Most of the cons of database-per-tenant without the per-tenant backup/restore cleanliness.
- Rejected because: it carries the costs of physical separation without the benefits, and MongoDB documentation specifically discourages it for SaaS multi-tenancy.

### API surface

#### 1. JWT claim (chosen)
- ✅ Tenant identity is signed — cannot be spoofed by a client.
- ✅ Carries naturally through to background jobs / Restate workflows that re-use the same JWT.
- ✅ Standard SaaS pattern; integrates with any OAuth 2.0 / OIDC identity provider.
- ❌ Requires the JWT verification path to be correct end-to-end; a key rotation bug becomes an authentication bug.

#### 2. Subdomain-based routing (rejected)
- ✅ Clean operationally — DNS resolves the tenant before the request reaches the application.
- ❌ Adds DNS / certificate management per tenant.
- ❌ Doesn't help non-browser clients (mobile apps, Restate workflows, internal jobs) and forces them to learn the tenant-to-subdomain mapping.
- Rejected because: the operational ceremony exceeds the benefit at this scale, and the subdomain doesn't survive into background contexts where the tenant identity must also be present.

#### 3. `X-Tenant-Id` header (rejected)
- ✅ Trivially simple to implement.
- ❌ Trivially spoofable — the header is set by the client and not anchored to authentication. A bug in authorization logic that trusts the header without cross-checking it against the authenticated principal is a tenancy bypass.
- Rejected because: carrying the tenant identity outside the authenticated envelope is a footgun and a recurring source of CVE-class bugs in SaaS systems.

## Confirmation

How we will know this decision is being followed:

- **Architecture test (commands/events/queries shape)**: an ArchUnit / Konsist test asserts that every class annotated with Axon's command, event, or query stereotype (or living in `domain/**/commands/`, `domain/**/events/`, `domain/**/queries/`) has a non-null `tenantId: TenantId` property.
- **Architecture test (read-model shape)**: an ArchUnit / Konsist test asserts that every Spring Data `@Document` class living in `adapter/out/readmodel/**` (or any `application/**/readmodels/**` package) has a `tenantId` field.
- **Architecture test (repository shape)**: an ArchUnit / Konsist test asserts that every `MongoRepository` / `Repository` interface in `adapter/out/readmodel/**` either takes `tenantId` as the first parameter on every query method, or uses `@Query` annotations whose JSON includes a `tenantId` filter. No `findAll` / `findById` / `findBy<X>` method may exist on a tenant-scoped repository without a `tenantId` filter.
- **Architecture test (no header-based tenant resolution)**: a forbidden-pattern scan asserts that no controller, filter, or interceptor reads a `X-Tenant-Id` (or similar) header — the only sanctioned source is the JWT claim, resolved by the dedicated authentication filter.
- **Integration test (cross-tenant isolation)**: a test bootstraps two tenants, writes commands and reads queries for each, and asserts that tenant A's queries return zero data belonging to tenant B for every read-model collection.
- **Manual review**: PRs that introduce a new command, event, query, read-model document, or read-repository method must show in the description how `tenantId` flows through the new path, including the failure mode if `tenantId` is missing.

## More Information

- [ADR-001 — Axon Framework 5 + Restate division of labor](ADR-001-axon-5-restate-division-of-labor.md)
- [INV-002 — Domain has no framework dependencies](../invariants/INV-002-domain-no-framework-dependencies.md)
- [INV-003 — Axon Server is the sole source of truth for event history](../invariants/INV-003-axon-server-sole-event-store.md)
- Project context: `docs/project-context.md`

<!-- Directives for edikt governance. Populated by /edikt:adr:compile. -->
[edikt:directives:start]: #
source_hash: 9c11bfbc83c9b054c1cec41c092fe78b73553e5a9a50fd86ece07e5de908d685
directives_hash: c8a67abdc3f8008cda91d8022fe7b9083f87be03d75037b28b4233d13ef3a931
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/domain/**"
  - "**/application/**"
  - "**/adapter/in/web/**"
  - "**/adapter/in/restate/**"
  - "**/adapter/out/readmodel/**"
scope:
  - planning
  - design
  - implementation
  - review
directives:
  - "The system MUST run with exactly one Axon Server context for the entire application. NEVER provision a per-tenant Axon Server context. (ref: ADR-002)"
  - "Every command, domain event, and query type MUST declare a non-null `tenantId: TenantId` property. NEVER define a command, event, or query type without a `tenantId` field. (ref: ADR-002)"
  - "Aggregate state MUST persist its owning `tenantId`, and every aggregate `@CommandHandler` method MUST validate that the incoming command's `tenantId` matches the aggregate's stored `tenantId` BEFORE emitting any event via `AggregateLifecycle.apply(...)`. A mismatch MUST raise a domain exception that prevents the state transition. (ref: ADR-002)"
  - "Every Spring Data `@Document` class for tenant-scoped read models MUST contain a `tenantId` field. Every compound index on tenant-scoped collections MUST lead with `tenantId`. (ref: ADR-002)"
  - "Every read-repository method that queries a tenant-scoped collection MUST accept `tenantId` as its first parameter and MUST apply it as a filter (either as a derived-query parameter or inside the `@Query` JSON). NEVER define `findAll`, `findById`, or `findBy<X>` methods on a tenant-scoped repository without a `tenantId` filter. (ref: ADR-002)"
  - "Every `@QueryHandler` that reads tenant-scoped data MUST extract `tenantId` from the query message and propagate it to every read-model lookup. NEVER perform a tenant-scoped read without filtering by `tenantId`. (ref: ADR-002)"
  - "Tenant identity on inbound HTTP requests MUST be resolved exclusively from a signed JWT `tid` claim, verified by an `OncePerRequestFilter` (or equivalent) at the `adapter/in/web/**` boundary, and exposed via a request-scoped `TenantContext` bean. NEVER read `tenantId` from a request body, query parameter, URL path segment, or unauthenticated HTTP header (e.g., `X-Tenant-Id`). (ref: ADR-002)"
  - "Restate handlers under `adapter/in/restate/**` MUST receive `tenantId` as part of their workflow input and MUST propagate it on every Axon command and query they dispatch. NEVER dispatch a tenant-scoped Axon command or query from a Restate handler without an explicit `tenantId`. (ref: ADR-002)"
reminders:
  - "Before defining a new command, event, query, or read-model `@Document` → add a non-null `tenantId: TenantId` property; tenancy is the project's hardest cross-cutting invariant (ref: ADR-002)"
  - "Before adding a read-repository or `@QueryHandler` method that touches tenant-scoped data → ensure `tenantId` is the first parameter and is applied as a filter on every lookup (ref: ADR-002)"
verification:
  - "[ ] Every command, domain event, and query type carries a non-null `tenantId: TenantId` field (ArchUnit / Konsist arch test) (ref: ADR-002)"
  - "[ ] Every `@Document` class for tenant-scoped read models has a `tenantId` field, and every compound index on those collections leads with `tenantId` (ref: ADR-002)"
  - "[ ] No controller, filter, or interceptor reads `tenantId` from a request body, query parameter, URL path, or `X-Tenant-Id` (or similar) HTTP header — JWT `tid` claim is the only sanctioned source (ref: ADR-002)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
