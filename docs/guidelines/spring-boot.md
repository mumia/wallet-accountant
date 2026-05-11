# Spring Boot Guidelines

**Purpose:** Keep the Spring layer thin, well-bounded, and operationally predictable — enforcing typed configuration, explicit profiles, hexagonal boundaries for `@RestController` and `@Document`, RFC 7807 error responses, structured logging, and narrow Actuator exposure so the framework's "convention over configuration" defaults never silently leak domain state, secrets, or write-side concerns Axon and Restate already own.

## Rationale

Spring Boot's strength — auto-configuration, classpath scanning, sensible defaults — is also the easiest way to dissolve architectural boundaries. Without explicit conventions, `@RestController`s end up referencing aggregates, `@Document` annotations slide into domain types, `@Value` annotations sprout across modules, and `default` profile behavior drifts between developer machines and production. The Axon + Restate + MongoDB read-model architecture means Spring is **not** the system of record for events (Axon Server is) or for durable workflows (Restate is). This guideline pins Spring to its actual job in this project: dependency injection, the inbound HTTP adapter (`adapter/in/web`), the outbound read-model adapter (`adapter/out/readmodel`), externalized configuration, and observability — and keeps every other concern out of the Spring layer.

The rules below also harden the operational surface: typed `@ConfigurationProperties` instead of scattered `@Value`, named profiles instead of implicit `default`, `ProblemDetail` responses instead of per-controller try/catch, slice tests instead of full-context bootstraps, whitelisted Actuator endpoints instead of `*`. Each rule maps directly to a category of incident this team can avoid by making the safe path the only path.

## Rules

- MUST bind external configuration via typed `@ConfigurationProperties` classes with constructor binding (`@ConfigurationProperties(prefix = "...")` on an immutable `data class`). NEVER scatter `@Value("${...}")` annotations for the same logical config block, and NEVER read `System.getenv` / `System.getProperty` directly from application code — go through `Environment` or a typed config class.
- MUST inject secrets (database passwords, API keys, OAuth client secrets, JWT signing keys) through environment variables resolved by Spring's property placeholder, or via a dedicated secrets backend. NEVER commit secret values to `application.yml`, profile-specific YAML, source code, fixtures, or any other tracked file.
- MUST name every Spring profile explicitly (`dev`, `test`, `staging`, `prod`) and gate environment-specific beans with `@Profile`. NEVER rely on the implicit `default` profile in a deployed environment — every running instance MUST set `SPRING_PROFILES_ACTIVE` to a known profile.
- MUST place every `@RestController` / `@Controller` class under the `adapter/in/web` package. NEVER let HTTP annotations (`@RestController`, `@Controller`, `@RequestMapping`, `@GetMapping`, `@PostMapping`, etc.) appear on classes in the domain or application layer — controllers are an inbound adapter and MUST translate to and from application-layer commands and queries at the boundary.
- MUST declare HTTP request and response bodies as dedicated Kotlin `data class` DTOs owned by the controller package, with explicit Jackson contracts (`@JsonProperty`, `@JsonInclude`) only where the wire name or null behavior diverges from the property. NEVER expose domain entities, aggregates, Axon commands or events, or `@Document` types directly from a controller endpoint.
- MUST validate request bodies with `@Valid` plus Bean Validation 3.x annotations (`@NotBlank`, `@Size`, `@Email`, etc.) and surface validation and domain failures through a single `@RestControllerAdvice` that returns RFC 7807 `ProblemDetail` responses. NEVER write per-controller `try/catch` blocks for validation or expected domain failures.
- NEVER perform business work, network calls, blocking I/O, or event publication inside `@PostConstruct`, bean constructors, `ApplicationContextInitializer`s, or `BeanFactoryPostProcessor`s. Move startup work into `ApplicationRunner` / `CommandLineRunner` or an `@EventListener(ApplicationReadyEvent::class)` so failures are observable and the container can fail fast.
- MUST keep Spring Data Mongo `@Document` types and `MongoRepository` / `ReactiveMongoRepository` interfaces under `adapter/out/readmodel` only. NEVER let a domain or application class reference Spring Data Mongo annotations (`@Document`, `@Field`, `@Id`, `@Indexed`, `@DBRef`, `@CompoundIndex`) — read models are projections owned by the outbound adapter.
- MUST declare every Mongo index explicitly via `@Indexed` / `@CompoundIndex` on the `@Document` AND apply indexes through Mongock change units (per ADR-007) under `**/adapter/out/readmodel/migrations/**`. NEVER use Mongobee, the Liquibase MongoDB extension, or a custom `IndexOps` startup runner — Mongock is the only sanctioned migration runner. NEVER set `spring.data.mongodb.auto-index-creation=true` in any profile — auto-creation hides index drift and surprises ops.
- MUST log via SLF4J — either `LoggerFactory.getLogger(...)` or `KotlinLogging.logger {}`. NEVER use `println`, `System.out.println`, `System.err.println`, `printStackTrace()`, or `java.util.logging` from application or test-runtime code.
- MUST whitelist Actuator endpoints by name in `management.endpoints.web.exposure.include` (e.g., `health,info,metrics,prometheus`). NEVER set the value to `*`, and NEVER expose `env`, `configprops`, `heapdump`, `threaddump`, `mappings`, `loggers`, or `beans` on a public-facing listener — sensitive endpoints MUST sit on a separate management port (`management.server.port`) or behind authentication.
- MUST use the narrowest Spring test slice for the layer under test — `@WebMvcTest` for controllers, `@DataMongoTest` for Mongo read models, `@JsonTest` for serialization, `@RestClientTest` for outbound HTTP clients. NEVER reach for `@SpringBootTest` when a slice would suffice; full-context tests are reserved for cross-layer smoke tests and consumer-driven contract tests.

## When NOT to apply

These rules do not apply to:

- **Spring's own auto-generated test scaffolding** — the smoke test produced by `start.spring.io` (`contextLoads()`) and any auto-generated configuration metadata. Replace it with real slice tests as soon as real components exist.
- **Internal-only management listeners** firewalled from public traffic. The Actuator exposure rule still applies in spirit (whitelist by name, never `*`), but the *set* of whitelisted endpoints MAY widen on a private listener — `env` and `configprops` are acceptable on an admin port that is never reachable from the internet.
- **Migration scripts and ops tooling under a clearly marked profile** (e.g., `tools`, `migration`). Startup work that is normally forbidden inside `@PostConstruct` is acceptable in a `CommandLineRunner` *gated by such a profile*, because the profile makes the side effect explicit and the run intentional.

These three exceptions are the only legitimate ones. "It works fine on my machine without setting `SPRING_PROFILES_ACTIVE`", "we'll move the controller later", and "the secret is only in the dev YAML" are not exceptions — they are the exact failure modes these rules exist to prevent.

---

*Created by edikt:guideline — 2026-04-26*

<!-- Directives for edikt governance. Populated by /edikt:guideline:compile. -->
[edikt:directives:start]: #
source_hash: a8727d6d6ed82728dea791533d6277f799554d7c870949dd3d81a9fc1dd6f30c
directives_hash: 8d88b2e2cff6f688f7dc617488f592111d5edbed233373a1f31268e696ffc7bf
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/*.kts"
  - "**/application*.yml"
  - "**/application*.yaml"
  - "**/application*.properties"
  - "**/bootstrap*.yml"
scope:
  - implementation
  - review
directives:
  - "MUST bind external configuration via typed `@ConfigurationProperties` classes with constructor binding (`@ConfigurationProperties(prefix = \"...\")` on an immutable `data class`). NEVER scatter `@Value(\"${...}\")` annotations for the same logical config block, and NEVER read `System.getenv` / `System.getProperty` directly from application code — go through `Environment` or a typed config class. (ref: spring-boot)"
  - "MUST inject secrets (database passwords, API keys, OAuth client secrets, JWT signing keys) through environment variables resolved by Spring's property placeholder, or via a dedicated secrets backend. NEVER commit secret values to `application.yml`, profile-specific YAML, source code, fixtures, or any other tracked file. (ref: spring-boot)"
  - "MUST name every Spring profile explicitly (`dev`, `test`, `staging`, `prod`) and gate environment-specific beans with `@Profile`. NEVER rely on the implicit `default` profile in a deployed environment — every running instance MUST set `SPRING_PROFILES_ACTIVE` to a known profile. (ref: spring-boot)"
  - "MUST place every `@RestController` / `@Controller` class under the `adapter/in/web` package. NEVER let HTTP annotations (`@RestController`, `@Controller`, `@RequestMapping`, `@GetMapping`, `@PostMapping`, etc.) appear on classes in the domain or application layer — controllers are an inbound adapter and MUST translate to and from application-layer commands and queries at the boundary. (ref: spring-boot)"
  - "MUST declare HTTP request and response bodies as dedicated Kotlin `data class` DTOs owned by the controller package, with explicit Jackson contracts (`@JsonProperty`, `@JsonInclude`) only where the wire name or null behavior diverges from the property. NEVER expose domain entities, aggregates, Axon commands or events, or `@Document` types directly from a controller endpoint. (ref: spring-boot)"
  - "MUST validate request bodies with `@Valid` plus Bean Validation 3.x annotations (`@NotBlank`, `@Size`, `@Email`, etc.) and surface validation and domain failures through a single `@RestControllerAdvice` that returns RFC 7807 `ProblemDetail` responses. NEVER write per-controller `try/catch` blocks for validation or expected domain failures. (ref: spring-boot)"
  - "NEVER perform business work, network calls, blocking I/O, or event publication inside `@PostConstruct`, bean constructors, `ApplicationContextInitializer`s, or `BeanFactoryPostProcessor`s. Move startup work into `ApplicationRunner` / `CommandLineRunner` or an `@EventListener(ApplicationReadyEvent::class)` so failures are observable and the container can fail fast. (ref: spring-boot)"
  - "MUST keep Spring Data Mongo `@Document` types and `MongoRepository` / `ReactiveMongoRepository` interfaces under `adapter/out/readmodel` only. NEVER let a domain or application class reference Spring Data Mongo annotations (`@Document`, `@Field`, `@Id`, `@Indexed`, `@DBRef`, `@CompoundIndex`) — read models are projections owned by the outbound adapter. (ref: spring-boot)"
  - "MUST declare every Mongo index explicitly via `@Indexed` / `@CompoundIndex` on the `@Document` AND apply indexes through Mongock change units (per ADR-007) under `**/adapter/out/readmodel/migrations/**`. NEVER use Mongobee, the Liquibase MongoDB extension, or a custom `IndexOps` startup runner — Mongock is the only sanctioned migration runner. NEVER set `spring.data.mongodb.auto-index-creation=true` in any profile — auto-creation hides index drift and surprises ops. (ref: spring-boot)"
  - "MUST log via SLF4J — either `LoggerFactory.getLogger(...)` or `KotlinLogging.logger {}`. NEVER use `println`, `System.out.println`, `System.err.println`, `printStackTrace()`, or `java.util.logging` from application or test-runtime code. (ref: spring-boot)"
  - "MUST whitelist Actuator endpoints by name in `management.endpoints.web.exposure.include` (e.g., `health,info,metrics,prometheus`). NEVER set the value to `*`, and NEVER expose `env`, `configprops`, `heapdump`, `threaddump`, `mappings`, `loggers`, or `beans` on a public-facing listener — sensitive endpoints MUST sit on a separate management port (`management.server.port`) or behind authentication. (ref: spring-boot)"
  - "MUST use the narrowest Spring test slice for the layer under test — `@WebMvcTest` for controllers, `@DataMongoTest` for Mongo read models, `@JsonTest` for serialization, `@RestClientTest` for outbound HTTP clients. NEVER reach for `@SpringBootTest` when a slice would suffice; full-context tests are reserved for cross-layer smoke tests and consumer-driven contract tests. (ref: spring-boot)"
reminders:
  - "Before binding configuration → use a typed `@ConfigurationProperties` class; never scatter `@Value` or read `System.getenv` directly (ref: spring-boot)"
  - "Before adding a `@RestController` or `@Document` → place it under the correct hexagonal adapter (`adapter/in/web` or `adapter/out/readmodel`) (ref: spring-boot)"
verification:
  - "[ ] No `@Value(\"${...}\")` annotations outside a `@ConfigurationProperties` class (ref: spring-boot)"
  - "[ ] No `@RestController` / `@Controller` / `@Document` annotations outside the corresponding `adapter/in/web/**` or `adapter/out/readmodel/**` packages (ref: spring-boot)"
  - "[ ] No `management.endpoints.web.exposure.include: \"*\"` and no exposure of `env`/`configprops`/`heapdump`/`threaddump` on any public listener in `application*.yml` (ref: spring-boot)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
