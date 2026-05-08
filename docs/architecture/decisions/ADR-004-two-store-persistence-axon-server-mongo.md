---
status: accepted
date: 2026-05-06
decision-makers: [Miguel Manso]
consulted: []
informed: []
supersedes: null
---

# ADR-004: Two-store persistence — Axon Server for events, MongoDB for read models

## Context and Problem Statement

wallet-accountant is event-sourced (per [INV-003](../invariants/INV-003-axon-server-sole-event-store.md), which already pins **Axon Server** as the sole event store) and serves queries from materialized read models. That leaves three architectural choices that interact:

1. **Which event store backs the write side?** Axon Server is named in INV-003, but Axon Framework 5 supports several `EventStorageEngine` implementations — `JpaEventStorageEngine` over JDBC (typically Postgres or MySQL), `MongoEventStorageEngine` over MongoDB, `InMemoryEventStorageEngine`, and the native Axon Server protocol. INV-003 says "Axon Server" but doesn't yet rule out the alternatives in build/config terms — a misconfigured `EventStorageEngine` bean could quietly turn the project into a JPA-backed event store while still appearing to honor INV-003 at the API level.

2. **Where do read models live?** CQRS naturally separates write and read storage, but the read store still has to be picked: PostgreSQL, MongoDB, Elasticsearch, the same store as events, etc.

3. **One physical store or two?** It is technically possible to use a single store for everything (e.g., Postgres for both events and read models, or Mongo for both via `MongoEventStorageEngine`). The operational simplicity of "one DB to run" tugs against the architectural separation of concerns.

The framing question for this ADR: how should the persistence layer be split, and what stops a future change from collapsing it into a single store or sprawling it into a third?

## Decision Drivers

- **Honor [INV-003](../invariants/INV-003-axon-server-sole-event-store.md) at the build/config layer**, not just at the API layer. The directive that "Axon Server is the sole event store" must be enforceable by greppable code patterns and dependency-graph rules, not just by API discipline.
- **Match each store to its workload.** Event log: append-only, read-by-offset/aggregate. Read models: random-access by tenant-scoped key, denormalized, droppable and rebuildable. Different access patterns warrant different stores.
- **Compatibility with [ADR-001](ADR-001-axon-5-restate-division-of-labor.md).** Axon Framework 5's Dynamic Consistency Boundary (DCB) facility is the consistency mechanism for cross-aggregate operations. DCB is documented and exercised on Axon Server's native protocol; support and behavior on the `JpaEventStorageEngine` and `MongoEventStorageEngine` paths is not verified for this project and would require validation before either could be relied on for cross-aggregate consistency.
- **Operational simplicity for a single-maintainer personal project (per [ADR-003](ADR-003-single-module-gradle-hexagonal.md))** — but not at the cost of the architectural guarantees above.
- **Local single-host deployment.** wallet-accountant runs locally — both stores execute as containers on the developer's own machine alongside the Spring Boot app and Restate. There is no managed-service tier, no cloud-provider backup, and no horizontal scale-out. The decision must be cheap to operate on one host and easy to bring up with a single `docker compose` invocation.
- **Cap the persistence surface.** Adding a third store (Postgres, Elasticsearch, Cassandra, Redis-as-primary, etc.) should require an ADR amendment, not just a Spring starter dependency added in a feature PR.

## Considered Options

1. **Axon Server (events) + MongoDB (read models)** — two purpose-built stores, each chosen for its workload.
2. **Single PostgreSQL for both** — Axon's `JpaEventStorageEngine` backed by Postgres + read models in the same Postgres. One DB to operate.
3. **Single MongoDB for both** — Axon's `MongoEventStorageEngine` backed by Mongo + read models in the same Mongo. One DB to operate.
4. **Axon Server (events) + Elasticsearch (read models)** — same write side, ES for read-side search and aggregations.
5. **Axon Server (events) + PostgreSQL (read models)** — same write side, relational read models.

## Decision Outcome

Chosen option: **Axon Server (events) + MongoDB (read models)**, because each store is purpose-built for its workload, the choice honors INV-003 and ADR-001 cleanly, and freezing the persistence surface to exactly two stores prevents the kind of accretion that turns "we'll add Postgres for one feature" into permanent dual-write complexity.

The decision lands as the following hard rules:

- The event store MUST be Axon Server, accessed via Axon Framework's native protocol. NEVER configure or instantiate `JpaEventStorageEngine`, `JdbcEventStorageEngine`, `MongoEventStorageEngine`, `InMemoryEventStorageEngine`, or any other `EventStorageEngine` implementation under `src/main/**`. The `InMemoryEventStorageEngine` is permitted only in `src/test/**` test fixtures.
- Read-model persistence MUST be MongoDB. NEVER persist read models to PostgreSQL, MySQL, Elasticsearch, Cassandra, Redis (as a primary store), DynamoDB, or any other database. The project has exactly two persistent stores: Axon Server (events) and MongoDB (read models).
- The Spring Data persistence starter in `gradle/libs.versions.toml` and every `build.gradle.kts` MUST be limited to `org.springframework.boot:spring-boot-starter-data-mongodb`. NEVER add `spring-boot-starter-data-jpa`, `spring-boot-starter-data-jdbc`, `spring-boot-starter-data-r2dbc`, `spring-boot-starter-data-elasticsearch`, `spring-boot-starter-data-cassandra`, `spring-boot-starter-data-redis` (for persistence), `spring-boot-starter-data-neo4j`, or any equivalent persistence starter.
- Persistence drivers in `gradle/libs.versions.toml` and every `build.gradle.kts` MUST be limited to `org.mongodb:*` (for read models) and Axon Framework / Axon Server client coordinates (for events). NEVER add `org.postgresql:postgresql`, `mysql:mysql-connector-java`, `co.elastic.clients:*`, `com.datastax.oss:*`, JDBC drivers for any other RDBMS, or any other persistence client library.
- This ADR does NOT govern caches. In-process caching (Caffeine, Spring's `@Cacheable`) and remote caches that are explicitly transient (Redis used as a cache only, with `spring-boot-starter-data-redis-reactive` strictly forbidden as a *persistence* path) are out of scope and remain available — but adding any cache that becomes a system of record reopens this ADR.

### Consequences

**Positive:**
- Each store does what it is good at: Axon Server is purpose-built for append-only event streams with tracking tokens, replication, and DCB support; Mongo is well-suited to denormalized, tenant-scoped, document-shaped materialized views.
- Read models can be wiped and rebuilt from offset zero without touching the event log — physical separation makes this safe.
- The persistence surface is closed. A new feature cannot quietly add Postgres "just for this one table" — it requires an ADR amendment.
- Honors INV-003 not only at the API layer but at the build/config layer: the wrong `EventStorageEngine` cannot be silently introduced, because the dependency tree forbids the supporting drivers and starters.
- Both stores run as local Docker containers (Axon Server SE image + an official MongoDB image) alongside the Spring Boot app and Restate. The development setup is `docker compose up`; backups are local volume snapshots; no managed-service tier is in the loop. At <10 tenants (per [ADR-002](ADR-002-multi-tenant-isolation.md)), the combined memory footprint of two stores is well within a developer-grade host.

**Negative:**
- Two stores to operate, monitor, back up, and tune. On a single-host local deployment this is two containers and two volume snapshots, but the operational concept count is still two.
- No cross-store transactions. Projecting an event into the read model is an *eventually consistent* operation. Acceptable — the existing CQRS architecture and INV-003 already require this.
- If a future feature genuinely needs full-text search or relational analytics that Mongo cannot serve well, the answer is not "add Elasticsearch" but "amend ADR-004 explicitly." That friction is intentional.
- **Reversibility cost is real and not zero.** If this decision turns out wrong and the project needs Postgres or Elasticsearch later, the unwind requires: an amendment to this ADR, removing the dependency-tree CI scan and the forbidden-pattern grep, updating `governance.md`'s verification checklist, relaxing the architecture test, and migrating any existing read-model collections. None of those are mechanical refactors — each is a deliberate governance edit. The friction is intentional, but it is paid in governance churn, not just in code.
- Network round-trips: queries that join across read-model collections happen client-side or via Mongo's `$lookup`. No SQL `JOIN` semantics.

**Neutral:**
- The application connects to Axon Server (via Axon's client), and to MongoDB (via Spring Data Mongo). Two client configurations, two health checks, two backup strategies.
- Caches (Caffeine, Spring `@Cacheable`, Redis-as-cache) remain a separate concern. This ADR does not forbid them — but a Redis or other cache that becomes a system of record falls under this ADR and requires an amendment.

## Pros and Cons of the Options

### 1. Axon Server (events) + MongoDB (read models) — chosen

- ✅ Each store optimized for its workload.
- ✅ Axon Server is AxonIQ's first-class event store with documented DCB support — matches [ADR-001](ADR-001-axon-5-restate-division-of-labor.md).
- ✅ Both run as local Docker containers alongside the Spring Boot app and Restate; `docker compose up` covers the entire local-dev environment, with no cloud-tier dependency.
- ✅ Document model fits denormalized, tenant-scoped read models per [ADR-002](ADR-002-multi-tenant-isolation.md).
- ❌ Two stores to operate, monitor, and back up.
- ❌ No cross-store transactions; projection lag is observable.

### 2. Single PostgreSQL for both (`JpaEventStorageEngine` + Postgres read models)

- ✅ One DB to operate, one backup strategy, one connection pool.
- ✅ Strong relational semantics on the read side, useful when queries cross many entities relationally.
- ❌ JpaEventStorageEngine has measurably higher write latency than Axon Server's native protocol; tracking-processor reads are heavier; clustering and replication become Postgres's responsibility, not Axon Server's.
- ❌ DCB on `JpaEventStorageEngine` is not verified for this project. AxonIQ's documentation and DCB references centre on Axon Server's native protocol; using JPA-backed storage would require validating that DCB semantics actually hold there before relying on the primitive [ADR-001](ADR-001-axon-5-restate-division-of-labor.md) commits us to. Adopting an unverified path for a load-bearing consistency mechanism is a risk we don't need to take.
- ❌ Read-side fit: read models are denormalized projections (an account with its embedded scheduled-transactions, an aggregated monthly view), not relational entities. Postgres works, but the document model fits better.
- **Rejected because:** trades away Axon Server's purpose-built event-store performance and DCB confidence for an operational-simplicity gain that is small at this scale (Axon Server SE is one container) and that does not address the read-side fit.

### 3. Single MongoDB for both (`MongoEventStorageEngine` + Mongo read models)

- ✅ One DB to operate.
- ✅ Document model on the read side.
- ❌ `MongoEventStorageEngine` is a legacy / less-favored path; AxonIQ's documentation and roadmap centre on Axon Server. DCB support on the Mongo storage engine has not been verified for this project, and adopting it for [ADR-001](ADR-001-axon-5-restate-division-of-labor.md)'s cross-aggregate consistency primitive without that verification is a risk.
- ❌ MongoDB is not optimized for append-only, high-throughput event log workloads — write amplification on indexed events, oplog pressure, replica-set lag during catch-up.
- ❌ Mixing the event log and read-model collections in the same logical database makes "drop and rebuild read models" risky — operationally, one wrong drop deletes events.
- **Rejected because:** the operational-simplicity gain is small, the event-store fit is poor, and the DCB story is weaker than Axon Server's. Worst of both worlds.

### 4. Axon Server (events) + Elasticsearch (read models)

- ✅ Full-text search, aggregations, time-series analytics out of the box.
- ✅ Axon Server retained for the write side (no DCB regression).
- ❌ Heavy local-host footprint: Elasticsearch is memory-hungry and JVM-tuned. Running ES alongside Axon Server, MongoDB, Restate, and the Spring Boot app on a single developer-grade host (per the local-deployment driver above) eats into the RAM budget without giving back proportional value for this project's actual read patterns.
- ❌ Eventually-consistent by default (1s refresh interval); fragile durability story for a primary read store.
- ❌ wallet-accountant's read patterns are point lookups by tenantId + accountId, list views by tenantId, monthly aggregates — none of which need ES's strengths.
- **Rejected because:** ES's superpowers (full-text, search relevance) don't match this project's read needs, and the infra cost is meaningful.

### 5. Axon Server (events) + PostgreSQL (read models)

- ✅ Same DCB-friendly write side as the chosen option.
- ✅ Strong typing, well-understood SQL access patterns.
- ❌ Read-models are denormalized document-shaped projections; Postgres works but is a less natural fit than Mongo.
- ❌ Adds JDBC and JPA into the dependency surface for read-side use that Mongo handles equally well.
- **Rejected because:** Mongo fits the projection shape better, has lighter operational footprint at this scale, and stays compatible with [ADR-002](ADR-002-multi-tenant-isolation.md)'s row-level filter approach without adding a second persistence dialect.

## Confirmation

How we will know this decision is being followed:

- **Forbidden-pattern scan (event storage engines)**: a CI step `grep -RE '(JpaEventStorageEngine|JdbcEventStorageEngine|MongoEventStorageEngine|InMemoryEventStorageEngine)' src/main` returns zero matches. Every Axon `EventStorageEngine` reference under `src/main/**` is forbidden — the application connects to Axon Server, not to a custom storage engine bean.
- **Dependency-tree scan (Spring Data starters)**: a CI step inspects the resolved runtime classpath (`./gradlew dependencies --configuration runtimeClasspath`) and asserts the only Spring Data starter present is `spring-boot-starter-data-mongodb`. Any other `spring-boot-starter-data-*` triggers CI failure.
- **Dependency-tree scan (persistence drivers)**: same task asserts no `org.postgresql:postgresql`, `mysql:mysql-connector-java`, `co.elastic.clients:*`, `com.datastax.oss:*`, or other persistence-client library is present in `runtimeClasspath`.
- **Architecture test**: an ArchUnit / Konsist test asserts that `MongoTemplate` / `MongoRepository` types are referenced only from classes under `**/adapter/out/readmodel/**`. No application or domain class instantiates a Mongo client directly.
- **Manual review**: any PR that adds a Spring Data starter, a JDBC driver, an Elasticsearch / Cassandra / Redis client, or a non-Axon-Server `EventStorageEngine` bean MUST be rejected on sight unless it is accompanied by an amendment to this ADR.

## More Information

- [INV-003 — Axon Server is the sole source of truth for event history](../invariants/INV-003-axon-server-sole-event-store.md)
- [ADR-001 — Axon Framework 5 + Restate division of labor](ADR-001-axon-5-restate-division-of-labor.md)
- [ADR-002 — Multi-tenant isolation strategy](ADR-002-multi-tenant-isolation.md)
- [ADR-003 — Single-module Gradle layout for hexagonal architecture](ADR-003-single-module-gradle-hexagonal.md)
- Project context: `docs/project-context.md`
- Axon Server documentation: https://docs.axoniq.io
- Spring Data MongoDB reference: https://docs.spring.io/spring-data/mongodb/reference/

<!-- Directives for edikt governance. Populated by /edikt:adr:compile. -->
[edikt:directives:start]: #
source_hash: 9954820ea0127c06a171aa3d50194955a152bb203974e7ddbb834fe0c10a7064
directives_hash: f519e5355121c426247ff0dfa8fee414d5366b192e1cd551465e86b40f556c2a
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/*.kts"
  - "gradle/libs.versions.toml"
  - "**/application*.yml"
  - "**/application*.yaml"
scope:
  - planning
  - design
  - implementation
  - review
directives:
  - "The event store MUST be Axon Server, accessed via Axon Framework's native protocol. NEVER configure or instantiate `JpaEventStorageEngine`, `JdbcEventStorageEngine`, `MongoEventStorageEngine`, `InMemoryEventStorageEngine`, or any other `EventStorageEngine` implementation under `src/main/**`. The `InMemoryEventStorageEngine` is permitted only in `src/test/**` test fixtures. (ref: ADR-004)"
  - "Read-model persistence MUST be MongoDB. NEVER persist read models to PostgreSQL, MySQL, Elasticsearch, Cassandra, Redis (as a primary store), DynamoDB, or any other database — wallet-accountant has exactly two persistent stores: Axon Server (events) and MongoDB (read models). (ref: ADR-004)"
  - "The Spring Data persistence starter in `gradle/libs.versions.toml` and every `build.gradle.kts` MUST be limited to `org.springframework.boot:spring-boot-starter-data-mongodb`. NEVER add `spring-boot-starter-data-jpa`, `spring-boot-starter-data-jdbc`, `spring-boot-starter-data-r2dbc`, `spring-boot-starter-data-elasticsearch`, `spring-boot-starter-data-cassandra`, `spring-boot-starter-data-redis` (for persistence), `spring-boot-starter-data-neo4j`, or any equivalent persistence starter. (ref: ADR-004)"
  - "Persistence drivers in `gradle/libs.versions.toml` and every `build.gradle.kts` MUST be limited to `org.mongodb:*` (for read models) and Axon Framework / Axon Server client coordinates (for events). NEVER add `org.postgresql:postgresql`, `mysql:mysql-connector-java`, `co.elastic.clients:*`, `com.datastax.oss:*`, JDBC drivers for any other RDBMS, or any other persistence client library. (ref: ADR-004)"
  - "This ADR governs persistent stores only. Caches that are explicitly transient — Caffeine, Spring `@Cacheable` with an in-memory backend, Redis used solely as a cache — remain available, but ANY cache that becomes a system of record (i.e., its data cannot be regenerated from Axon Server events or recomputed on demand) reopens this ADR and requires an amendment before being added. (ref: ADR-004)"
reminders:
  - "Before adding a Spring Data starter or persistence driver to `gradle/libs.versions.toml` or any `build.gradle.kts` → confirm the coordinate is `spring-boot-starter-data-mongodb` or an `org.mongodb:*` / Axon coordinate; never JPA/JDBC/Elasticsearch/Cassandra/Postgres/etc. (ref: ADR-004)"
  - "Before declaring an Axon `EventStorageEngine` bean in `src/main/**` → don't; the application MUST connect to Axon Server via Axon Framework's native protocol, never via `JpaEventStorageEngine`, `JdbcEventStorageEngine`, `MongoEventStorageEngine`, or `InMemoryEventStorageEngine` (ref: ADR-004)"
verification:
  - "[ ] Resolved `runtimeClasspath` (from `./gradlew dependencies --configuration runtimeClasspath`) contains `spring-boot-starter-data-mongodb` and no other `spring-boot-starter-data-*` persistence starter (ref: ADR-004)"
  - "[ ] No code under `src/main/**` references `JpaEventStorageEngine`, `JdbcEventStorageEngine`, `MongoEventStorageEngine`, or `InMemoryEventStorageEngine` (ref: ADR-004)"
  - "[ ] No JDBC driver, Elasticsearch client, Cassandra client, or other non-Mongo / non-Axon persistence library is present in the resolved `runtimeClasspath` (ref: ADR-004)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
