# Research: Account Domain

**Feature**: 001-account-domain
**Date**: 2026-03-20

## R1: Axon Framework 5 Entity Pattern

**Decision**: Use AF5's `@EventSourcedEntity` annotation pattern with static `@CommandHandler` + `EventAppender` for aggregate creation.

**Rationale**: AF5 replaces AF4's `@Aggregate`/`@AggregateIdentifier` with `@EventSourcedEntity(tagKey=...)` and `@EntityCreator`. Creation commands use a static `@CommandHandler` in a companion object with `EventAppender.append()` instead of constructor-based handlers with `AggregateLifecycle.apply()`. This is the canonical AF5 approach documented in the project's af5-patterns memory.

**Alternatives considered**:
- AF4 patterns (`@Aggregate`, constructor `@CommandHandler`) — incompatible with AF5
- Spring's `@EventSourced` stereotype — combines entity + Spring component; viable but less explicit

## R2: Value Object Representation for Enums

**Decision**: Use Kotlin `enum class` for BankName, AccountType, and Currency. Each enum entry stores a display name.

**Rationale**: These are closed, fixed sets defined in the spec. Kotlin enums are immutable, serializable to JSON (by name), and enforce exhaustive `when` matching. Constitution principle #9 (Immutability) and #12 (JSON Serializability) are satisfied natively.

**Alternatives considered**:
- Sealed classes — unnecessary flexibility for fixed sets
- String constants — no type safety, no exhaustive matching

## R3: Money Value Object Precision

**Decision**: Use `BigDecimal` with scale 2 (HALF_UP rounding) wrapped in an immutable `Money` value object. Pair with `Currency` enum.

**Rationale**: `BigDecimal` avoids floating-point precision errors for financial amounts. Scale 2 enforced at construction satisfies FR-008. The wrapper ensures the precision invariant is always maintained. For JSON serialization, `BigDecimal` is natively supported by Jackson.

**Alternatives considered**:
- `Long` (cents) — simpler but loses semantic clarity and complicates display
- `Double` — unacceptable precision loss for financial data

## R4: Date Value Object

**Decision**: Use `java.time.LocalDate` wrapped in an immutable `Date` value object. Serialize as ISO-8601 string (YYYY-MM-DD).

**Rationale**: `LocalDate` provides day precision without timezone complexity. The spec says "UTC timezone" but since we only need day precision, `LocalDate` is sufficient — UTC is the serialization context, not runtime. Jackson's JavaTimeModule handles `LocalDate` ↔ ISO string serialization. Month and Year can be derived from `LocalDate` for the Account's month/year tracking (FR-003).

**Alternatives considered**:
- `java.time.ZonedDateTime` — unnecessary precision (time component not needed)
- `java.time.Instant` — requires timezone conversion for day extraction
- Custom date type — unnecessary when `LocalDate` fits perfectly

## R5: AccountId Strategy

**Decision**: Use `kotlin.uuid.Uuid` wrapped in an immutable `AccountId` value object. Requires `@JsonValue`/`@JsonCreator` for Jackson serialization.

**Rationale**: UUIDs provide globally unique, collision-resistant identifiers. The project already uses `kotlin.uuid.Uuid` (Kotlin 2.3+) and has documented the Jackson serialization workaround. The `AccountId` wrapper provides type safety and follows the "shared identifier pattern" mentioned in the spec.

**Alternatives considered**:
- `java.util.UUID` — works but the project standardized on `kotlin.uuid.Uuid`
- String — no type safety

## R6: Aggregate Creation Retry Interceptor

**Decision**: Implement a generic `MessageHandlerInterceptor<CommandMessage>` that catches duplicate aggregate ID exceptions and retries with a new ID. Uses an interface (`HasAggregateId`) to identify commands that support ID regeneration.

**Rationale**: FR-010 requires graceful handling of duplicate IDs. The CLAUDE.md specifies "generic aggregate creation retry interceptor that handles duplicate ID conflicts via an interface-based approach." AF5 interceptors use `interceptOnHandle` returning `MessageStream<*>`.

**Alternatives considered**:
- Per-command retry logic — violates DRY, not generic
- Client-side retry — leaks infrastructure concerns to callers

## R7: Project Build Setup

**Decision**: Gradle with Kotlin DSL. Dependencies: Axon Framework 5.0.3 (BOM), Spring Boot 3.5.3, axon-spring-boot-starter 5.0.3, Jackson, MongoDB driver. JDK 21. Kotlin 2.3.0.

**Rationale**: All versions verified in project memory. The `spring-boot-starter-web` is needed (not just `spring-boot-starter`) for `Jackson2ObjectMapperBuilderCustomizer`. No `axon-kotlin` or `axon-kotlin-test` exist for AF5.

**Alternatives considered**: None — tech stack is locked by constitution.
