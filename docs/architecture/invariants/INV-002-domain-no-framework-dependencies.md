# INV-002: Domain layer has no framework dependencies

**Date:** 2026-05-01
**Status:** Active

## Statement

The domain layer depends only on the Kotlin standard library, `java.time.*`, and other domain types — never on a framework, persistence engine, transport, or serialization library.

## Rationale

Hexagonal / DDD's value comes from the compile-time guarantee that business rules survive infrastructure change. Any framework, persistence, transport, or serialization import inside `domain/` couples the rules to a specific runtime — and the next migration breaks them. Review-time discipline drifts; only the dependency boundary holds the line. This invariant exists because once a `@Document` annotation appears on an entity "just this once", every future entity follows, and within months the domain is a Spring Data Mongo schema with extra ceremony.

## Consequences of violation

- Domain logic becomes inseparable from the chosen framework — Spring Boot deprecation cycles force domain rewrites and Kotlin / JVM upgrades have to wait on framework compatibility.
- Testing the domain requires the framework's runtime (Spring context boot, Mongo connection, Axon Server) — slower, flakier, harder to isolate, and dramatically more expensive in CI minutes.
- Replacing the persistence engine, transport, or DI framework requires touching every business class, not just the adapters that were supposed to absorb that change.
- The hexagonal model collapses into a layered Spring application with extra ceremony but none of the protection — the architectural cost paid up-front yields zero return.
- Loss of replayability and portability — the very reasons this project chose hexagonal in the first place.

## Implementation

The domain layer lives under `**/domain/**`. Domain types are plain Kotlin classes — `data class` for value objects, sealed hierarchies for outcome types, regular classes for aggregate roots — and import only from `kotlin.*`, `kotlin.collections.*`, `java.time.*` (where unavoidable for time and IDs), and other domain types in this project. Driven ports (interfaces consumed by the domain or application) are declared in the application layer, never in the domain layer; framework-bound implementations live under `adapter/out/**`. The hexagonal-ddd guideline (rules on domain isolation, port location, and adapter siblings) captures the architectural mechanism.

## Anti-patterns

- A `@Document` annotation on a domain entity "to make persistence simpler" — couples the domain to MongoDB's mapping rules and document-id strategy.
- `import org.springframework.context.event.EventListener` inside a domain aggregate — couples the domain to Spring's event bus and lifecycle.
- Jackson `@JsonProperty` on a value object "for the API" — the API DTO belongs in `adapter/in/web`, not in the domain.
- A `kotlinx.serialization.Serializable` annotation on a domain type "so we can store snapshots" — the snapshot mechanism is an adapter concern; the domain type stays clean and the adapter does the conversion.
- An HTTP client interface (`OkHttpClient`, `WebClient`, `HttpClient`) referenced from the domain "to fetch external data" — that's a driven port; declare the interface in `application/`, implement it in `adapter/out/http/`.
- A logger from a specific framework imported into a domain class — even `org.slf4j.Logger` belongs in adapters or services that wrap the domain, not inside it; pure domain code communicates via return values, not log statements.

## Enforcement

- **Automated (architecture test)**: an ArchUnit / Konsist / Modulint test asserts that no class under `**/domain/**` imports from `org.springframework.*`, `org.springframework.data.*`, `dev.restate.*`, `jakarta.servlet.*`, `com.fasterxml.jackson.*`, `kotlinx.serialization.*`, `org.axonframework.spring.*`, `org.hibernate.*`, OkHttp / WebClient packages, or any messaging-framework package. CI fails on any violation, before tests run.
- **Automated (lint)**: a Detekt rule blocks framework-annotation imports inside `domain/` packages and surfaces the violation at build time.
- **Manual**: PR reviewers visually inspect the import list of every changed file under `**/domain/**` against the forbidden-prefix list and reject offending imports outright.

<!-- Directives for edikt governance. Populated by /edikt:invariant:compile. -->
[edikt:directives:start]: #
source_hash: b80a9785dafde05f504583520eefcd179cf70d8dbfe1223dd83f49ba061dcb9d
directives_hash: 31c9f78b5e4cc9ee311e8d87c593243fe093a2c985a76a2ed5e25dc5199f553b
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/domain/**"
scope:
  - design
  - implementation
  - review
directives:
  - "Files under `**/domain/**` MUST depend only on the Kotlin standard library, `java.time.*`, and other domain types. NEVER import from Spring (`org.springframework.*`), Spring Data (`org.springframework.data.*`), Restate (`dev.restate.*`), Jakarta Servlet (`jakarta.servlet.*`), Jackson (`com.fasterxml.jackson.*`), `kotlinx.serialization.*`, Hibernate, OkHttp / WebClient, or any messaging-framework package inside `domain/`. (ref: INV-002)"
reminders:
  - "Before adding an import to a `domain/` file → confirm the package is `kotlin.*`, `java.time.*`, or another domain type — never a framework, persistence, transport, or serialization package (ref: INV-002)"
verification:
  - "[ ] No imports under `**/domain/**` from Spring, Spring Data, Restate, Jakarta Servlet, Jackson, kotlinx.serialization, Hibernate, or HTTP-client packages (ref: INV-002)"
  - "[ ] No framework annotations on classes under `**/domain/**` — no `@Document`, `@Component`, `@Service`, `@JsonProperty`, `@Serializable`, `@RestController`, etc. (ref: INV-002)"
  - "[ ] Architecture test (ArchUnit / Konsist / Modulint) for domain isolation is configured and passing in CI (ref: INV-002)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
