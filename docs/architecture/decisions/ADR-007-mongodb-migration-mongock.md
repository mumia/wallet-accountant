---
status: accepted
date: 2026-05-10
decision-makers: [Miguel Manso]
consulted: []
informed: []
supersedes: null
---

# ADR-007: MongoDB migration tooling — Mongock

## Context and Problem Statement

`docs/guidelines/spring-boot.md` already requires that every Mongo index be applied "through a migration tool (`mongock`, `mongobee`) or a startup `IndexOps` job gated behind a profile" and forbids `spring.data.mongodb.auto-index-creation=true` in staging/prod. That guideline is *permissive* — three options, take your pick — and it leaves a real decision unmade. Three concrete tools fit the slot, plus two non-tools that need to be ruled out:

1. **Mongock** — the actively-maintained successor to Mongobee. Annotation-based `@ChangeUnit` classes, lock collection for concurrent-startup safety, history collection for version tracking, Spring Boot 3 / Spring Data MongoDB 4 compatible, transactional change units on replica sets.
2. **Mongobee** — original Mongo-focused migration library, inspired by Liquibase. Largely unmaintained; no Spring Boot 3 support; Mongock is its supported continuation.
3. **Liquibase MongoDB extension** (`org.liquibase.ext:liquibase-mongodb`) — Liquibase ecosystem support, XML/YAML change logs, mature tooling outside Mongo. Heavier; the abstraction was designed for SQL DBs and shows the seams when applied to documents.
4. **Custom `IndexOperations` runner** — Spring Data's `MongoTemplate.indexOps()` invoked from an `ApplicationRunner` or `@EventListener(ApplicationReadyEvent::class)`. DIY ordering, locking, version tracking, rollback.
5. **Spring Data auto-index-creation** (`spring.data.mongodb.auto-index-creation=true`) — Spring infers indexes from `@Indexed` / `@CompoundIndex` annotations and creates them at startup. No history, no locking, silent surprises in production. The spring-boot guideline already forbids this for staging/prod.

Without a pinned choice the project would either freeze at "guideline says any of these is OK" (ambient ambiguity) or quietly drift toward the easiest path each engineer reaches for. For schema/index changes that survive across deploys, that drift is exactly the operational cost a migration tool exists to prevent.

How should we apply MongoDB index and schema migrations, and what stops a future PR from quietly switching tools?

## Decision Drivers

- **Active maintenance, Spring Boot 3 / Spring Data Mongo 4 compatible.** [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md) commits to Spring Boot's MongoDB starter; the migration tool must travel with that stack.
- **Operational safety primitives** — locking against concurrent startups (the local-deployment driver in [ADR-003](ADR-003-single-module-gradle-hexagonal.md) means a single instance, but `docker compose up` followed by a quick restart can race), versioned history of applied changes, idempotent re-runs.
- **Idiomatic Kotlin + Spring integration.** Annotations on classes, not external XML / YAML change logs.
- **Lightest operational footprint that meets the safety bar.** No external scheduler, no separate Liquibase JVM, no operator-only manual steps.
- **Compatibility with [INV-002](../invariants/INV-002-domain-no-framework-dependencies.md).** Migration code lives in the adapter layer (specifically `adapter/out/readmodel/migrations/`), never in `domain/` or `application/`.
- **Honors the spring-boot guideline's existing rules.** This ADR narrows the guideline's permissive option list rather than contradicting it; the guideline will be tightened to match.

## Considered Options

1. **Mongock** — annotation-based change units, lock + history collections, Spring Boot autoconfig.
2. **Mongobee** — older predecessor; not actively maintained.
3. **Liquibase MongoDB extension** — full Liquibase ecosystem, change logs in XML/YAML/JSON.
4. **Custom `IndexOperations` runner** — DIY in `ApplicationRunner` / `@EventListener(ApplicationReadyEvent::class)`.
5. **Spring Data `auto-index-creation=true`** — no migration tool at all; rely on `@Indexed` inference at startup.

## Decision Outcome

Chosen option: **Mongock**, because it ships the operational primitives we'd otherwise have to hand-build (locking, history, idempotency) in an annotation-based form that fits the existing Spring + Kotlin stack, while staying lighter than Liquibase and avoiding Mongobee's maintenance dead-end.

The decision lands as the following hard rules:

- The MongoDB migration tool MUST be **Mongock** (`io.mongock:mongock-springboot-v3:*` for Spring Boot 3, or its successor coordinate for Spring Boot 4+). NEVER add `com.github.mongobee:*`, `org.liquibase.ext:liquibase-mongodb:*`, or any other Mongo migration library to `gradle/libs.versions.toml` or any `build.gradle.kts` for production code paths.
- Migration units MUST be Mongock `@ChangeUnit` classes living under `**/adapter/out/readmodel/migrations/**`. Each `@ChangeUnit` MUST declare an `id`, an `order` (zero-padded numeric string for stable ordering), and an `author`. NEVER place change units inside `**/domain/**`, `**/application/**`, `**/adapter/in/**`, or `**/infrastructure/**`.
- Once a `@ChangeUnit` has been executed in any environment (developer machine, staging, production), its `execution` body MUST NEVER be modified — schema corrections proceed as a new `@ChangeUnit` with a higher `order`. NEVER edit historical change-unit code to "fix" or "amend" it; the immutability of executed migrations is the contract that makes Mongock's history collection trustworthy.
- `spring.data.mongodb.auto-index-creation` MUST be `false` (or absent — Spring Boot 3 defaults to false) in every `application*.yml` profile, including `dev`, `test`, `staging`, and `prod`. NEVER set this property to `true` in any tracked profile. Indexes MUST be applied exclusively through Mongock change units (which invoke `IndexOperations` or `MongoTemplate.indexOps()` inside the change-unit body).
- Mongock MUST run during application startup via the standard `MongockSpringBoot` auto-configuration. Custom invocation is permitted only in tests for fixture loading. NEVER trigger production migrations from manual `mongo` shell scripts, ad-hoc operator commands, or out-of-band runners — the running application is the migration runner.
- The Mongock lock and history collections MUST live in the **same MongoDB database** as the read models (per [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md), the application has exactly one Mongo database). NEVER configure Mongock to write its bookkeeping to a separate database, a separate cluster, or any non-Mongo store.

### Consequences

**Positive:**
- Concurrent-startup safety for free — the lock collection prevents two instances from racing on the same migration. Useful even for the single-host setup if `docker compose up` overlaps a restart.
- Versioned history of applied migrations — `mongockChangeLog` tells us exactly which change unit ran, when, in what duration, with what hash.
- Idempotent re-runs — running the application against an already-migrated database is a no-op.
- Annotation-based change units fit Kotlin + Spring idiomatically; no external change-log file to keep in sync.
- Spring Boot autoconfig means migrations are part of "start the app", not a separate ops step that someone forgets.

**Negative:**
- Mongock locks the project into a specific tool. Migrating to Liquibase or another runner later means re-encoding existing change units in the new tool's format AND reconciling the bookkeeping (Mongock's history collection vs the new tool's). Not free.
- Mongock's own dependency surface is meaningful (`mongock-bom`, transitive Spring beans). Less lean than a custom `IndexOps` runner — which is the whole reason we're not picking the custom runner.
- The `@ChangeUnit` immutability rule is a discipline that needs to hold in code review. A "small fix" to an already-executed change unit looks innocent but corrupts the history-collection contract.

**Neutral:**
- The Mongock library coordinate appears in `gradle/libs.versions.toml`. This is consistent with [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)'s "MongoDB-only persistence" surface — Mongock is a Mongo client, not a competing persistence store.
- Migration code becomes a subdirectory of `adapter/out/readmodel/`, which already houses the `@Document` classes the migrations operate on. Layer-first organisation per [ADR-003](ADR-003-single-module-gradle-hexagonal.md) is preserved.

## Pros and Cons of the Options

### 1. Mongock (chosen)

- ✅ Actively maintained; current Spring Boot 3 / Spring Data Mongo 4 support.
- ✅ Locking, history, idempotency, transactional change units (on replica sets) — the operational primitives that distinguish a migration tool from a startup script.
- ✅ Annotation-based change units → fits Kotlin + Spring idiomatically.
- ✅ Spring Boot autoconfig (`MongockSpringBoot`) means zero ceremony to wire up.
- ❌ Locks the project into a specific tool's history-collection format.
- ❌ Adds a non-trivial library + autoconfig surface compared to a hand-rolled runner.

### 2. Mongobee

- ✅ Original Mongo-migration library; mature design ideas Mongock inherited.
- ❌ Effectively unmaintained — no Spring Boot 3 support, no recent releases, the project's documentation explicitly recommends Mongock as the successor.
- ❌ Adopting an abandoned library is a known-bad path; tools that work today against deprecated APIs stop working tomorrow.
- **Rejected because:** the project Mongock superseded. Picking Mongobee in 2026 is choosing the predecessor when the successor is available, free, and addresses the same problem better.

### 3. Liquibase MongoDB extension

- ✅ Established Liquibase ecosystem; tooling, IDE plugins, change-log validators.
- ✅ Cross-database operability if the project ever needed a relational sidecar (which [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md) explicitly forbids without an amendment).
- ❌ Heavier dependency surface; Liquibase was designed for SQL and the abstraction shows when applied to documents.
- ❌ Change logs in XML/YAML/JSON instead of Kotlin annotations — breaks the "code reviews are diffable Kotlin" property the rest of the project enjoys.
- ❌ Cross-database operability is moot here per ADR-004's two-store closure.
- **Rejected because:** brings cross-DB capability we explicitly aren't using and pays for it in a heavier, less Kotlin-native surface.

### 4. Custom `IndexOperations` runner

- ✅ Lightest possible dependency surface — no third-party migration library at all.
- ✅ Total control over execution order and error handling.
- ❌ DIY locking against concurrent startups; the simple version (no lock) is a footgun the moment two instances start at once.
- ❌ DIY versioned history; the simple version (a custom collection of "applied" markers) is the start of writing Mongock badly.
- ❌ DIY idempotency; "create this index if it doesn't exist" is fine, "rename this field across 100k documents idempotently" is not, and the project will eventually need the latter.
- **Rejected because:** every problem the custom runner avoids by being small, Mongock has solved already. We'd end up reimplementing Mongock badly within a few migrations.

### 5. Spring Data `auto-index-creation=true` (no migration tool)

- ✅ Zero ceremony — annotate `@Indexed`, restart the app, indexes appear.
- ❌ No version tracking — the running database's index set silently drifts from what was annotated last release.
- ❌ No lock — multiple instances starting concurrently can race on `createIndex` calls.
- ❌ Already forbidden by `docs/guidelines/spring-boot.md` for staging/prod (and tightened to "never in any profile" by this ADR).
- **Rejected because:** explicitly out of bounds; the guideline already named auto-index-creation as the failure mode this ADR's chosen tool prevents.

## Confirmation

How we will know this decision is being followed:

- **Dependency-tree scan**: a CI step inspects `./gradlew dependencies --configuration runtimeClasspath` and asserts that exactly one Mongo-migration library coordinate is present, namespaced `io.mongock:*`. Any `com.github.mongobee:*` or `org.liquibase.ext:liquibase-mongodb:*` coordinate triggers CI failure.
- **Layer-location scan**: an ArchUnit / Konsist test asserts that every class annotated with `@ChangeUnit` lives in a package matching `**/adapter/out/readmodel/migrations/**`. Change units found anywhere else fail the build.
- **Profile scan**: a CI step greps every `application*.yml` for `spring.data.mongodb.auto-index-creation` and asserts the value is either absent or `false`. `true` in any tracked profile fails CI.
- **Change-unit immutability (manual)**: PR reviewers for any change touching `**/adapter/out/readmodel/migrations/**` MUST verify the diff is either (a) a new `@ChangeUnit` file with a higher `order`, or (b) a no-functional-change refactor (e.g., logging, comments) on a not-yet-executed change unit (i.e., not yet present in production's `mongockChangeLog`). NEVER allow edits to the `execution()` body of a change unit that has already been applied.
- **Manual review**: PRs that add a Mongo-migration library, a startup-time `IndexOperations` invocation outside a `@ChangeUnit`, or a `@PostConstruct` / `CommandLineRunner` that mutates the schema MUST be reviewed against this ADR and rejected unless they amend it.

## More Information

- [ADR-003 — Single-module Gradle layout for hexagonal architecture](ADR-003-single-module-gradle-hexagonal.md) — the layer-first organization that places change units under `adapter/out/readmodel/`.
- [ADR-004 — Two-store persistence — Axon Server for events, MongoDB for read models](ADR-004-two-store-persistence-axon-server-mongo.md) — the Mongo persistence surface this tooling operates against, and the no-other-DBs closure.
- [INV-003 — Axon Server is the sole source of truth for event history](../invariants/INV-003-axon-server-sole-event-store.md) — Mongock applies to read-model migrations only; the event log is owned by Axon Server and is not subject to Mongock change units.
- `docs/guidelines/spring-boot.md` — the guideline whose permissive `mongock | mongobee | IndexOps` choice this ADR narrows.
- Mongock documentation: https://docs.mongock.io
- Mongock Spring Boot integration: https://docs.mongock.io/v5/runner/springboot/index.html

<!-- Directives for edikt governance. Populated by /edikt:adr:compile. -->
[edikt:directives:start]: #
source_hash: a26b43d7666fc049a9fad99cb7dcb94493fe42ceb632a9f5ff0398988618d889
directives_hash: 2619769b809b907939e78ef87b0fbe5458a76942e6e6586837afb97ce0304975
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/application*.yml"
  - "**/application*.yaml"
  - "gradle/libs.versions.toml"
  - "**/adapter/out/readmodel/migrations/**"
scope:
  - planning
  - design
  - implementation
  - review
directives:
  - "The MongoDB migration tool MUST be Mongock (`io.mongock:mongock-springboot-v3:*` for Spring Boot 3, or its successor coordinate for Spring Boot 4+). NEVER add `com.github.mongobee:*`, `org.liquibase.ext:liquibase-mongodb:*`, or any other Mongo migration library to `gradle/libs.versions.toml` or any `build.gradle.kts` for production code paths. (ref: ADR-007)"
  - "Migration units MUST be Mongock `@ChangeUnit` classes living under `**/adapter/out/readmodel/migrations/**`. Each `@ChangeUnit` MUST declare an `id`, an `order` (zero-padded numeric string for stable ordering), and an `author`. NEVER place change units inside `**/domain/**`, `**/application/**`, `**/adapter/in/**`, or `**/infrastructure/**`. (ref: ADR-007)"
  - "Once a `@ChangeUnit` has been executed in any environment (developer machine, staging, production), its `execution` body MUST NEVER be modified — schema corrections proceed as a new `@ChangeUnit` with a higher `order`. NEVER edit historical change-unit code to `fix` or `amend` it; the immutability of executed migrations is the contract that makes Mongock's history collection trustworthy. (ref: ADR-007)"
  - "`spring.data.mongodb.auto-index-creation` MUST be `false` (or absent — Spring Boot 3 defaults to false) in every `application*.yml` profile, including `dev`, `test`, `staging`, and `prod`. NEVER set this property to `true` in any tracked profile. Indexes MUST be applied exclusively through Mongock change units. (ref: ADR-007)"
  - "Mongock MUST run during application startup via the standard `MongockSpringBoot` auto-configuration. Custom invocation is permitted only in tests for fixture loading. NEVER trigger production migrations from manual `mongo` shell scripts, ad-hoc operator commands, or out-of-band runners — the running application is the migration runner. (ref: ADR-007)"
  - "The Mongock lock and history collections MUST live in the same MongoDB database as the read models (per ADR-004, the application has exactly one Mongo database). NEVER configure Mongock to write its bookkeeping to a separate database, a separate cluster, or any non-Mongo store. (ref: ADR-007)"
reminders:
  - "Before changing a Mongo schema or index → create a new `@ChangeUnit` under `adapter/out/readmodel/migrations/` with a unique `id` and increasing `order`; never modify an executed change unit (ref: ADR-007)"
  - "Before configuring a Mongo migration runner or setting `spring.data.mongodb.auto-index-creation` → use Mongock only; never Mongobee, Liquibase Mongo, custom IndexOps, or auto-index-creation in any profile (ref: ADR-007)"
verification:
  - "[ ] Resolved `runtimeClasspath` contains exactly one Mongo-migration coordinate, namespaced `io.mongock:*`; no `com.github.mongobee:*`, `org.liquibase.ext:liquibase-mongodb:*`, or other migration tooling is present (ref: ADR-007)"
  - "[ ] Every `@ChangeUnit`-annotated class lives under `**/adapter/out/readmodel/migrations/**`; ArchUnit / Konsist test asserts no change units exist in `domain/`, `application/`, `adapter/in/`, or `infrastructure/` (ref: ADR-007)"
  - "[ ] Every `application*.yml` profile has `spring.data.mongodb.auto-index-creation` either unset or `false`; no profile sets it to `true` (ref: ADR-007)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
