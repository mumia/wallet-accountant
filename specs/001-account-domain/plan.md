# Implementation Plan: Account Domain

**Branch**: `001-account-domain` | **Date**: 2026-03-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-account-domain/spec.md`

## Summary

Implement the Account aggregate as the first domain entity in the Wallet Accountant application. This includes the Account event-sourced entity with a RegisterNewAccount command and NewAccountRegistered event, supporting value objects (AccountId, BankName, AccountType, Money, Currency, Date), a generic aggregate creation retry interceptor, the Gradle build setup, and Jackson serialization configuration. All built on Axon Framework 5, Spring Boot 3, and Kotlin 2.3.

## Technical Context

**Language/Version**: Kotlin 2.3.0 (stable `kotlin.uuid.Uuid` with `@OptIn` compiler flag)
**Primary Dependencies**: Axon Framework 5.0.3 (BOM), Spring Boot 3.5.3, axon-spring-boot-starter 5.0.3, Jackson
**Storage**: Axon Server (event store), MongoDB (read models — not used in this feature)
**Testing**: JUnit 5, Axon Test Fixture (`AxonTestFixture.with()`)
**Target Platform**: JVM 21, backend service
**Project Type**: Web service (backend only)
**Performance Goals**: N/A for this feature (domain model + command handling)
**Constraints**: All objects immutable, JSON-serializable, no frontend
**Scale/Scope**: Single aggregate, 1 command, 1 event, 6 value objects, 1 interceptor

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Status | Notes |
|---|-----------|--------|-------|
| 1 | Event Sourcing First | PASS | Account is event-sourced; NewAccountRegistered event persisted to Axon Server |
| 2 | CQRS Separation | PASS | Command side only in this feature; no query/projection side |
| 3 | Aggregate Design | PASS | Account protects a single consistency boundary; commands validated before events emitted |
| 4 | Test-Driven Development | PASS | Tests planned for aggregate, value objects, interceptor, and serialization |
| 5 | Simplicity | PASS | Minimal design: one aggregate, one command, one event, flat value objects |
| 6 | Hexagonal Architecture | PASS | Domain/Application/Adapter layers; interceptor in Application layer |
| 7 | Inversion of Control | PASS | Spring DI; dependencies injected, not instantiated directly |
| 8 | Dynamic Consistency Boundary | N/A | Single aggregate, no cross-aggregate operations |
| 9 | Immutability | PASS | All data classes immutable; aggregate state set via event sourcing handlers |
| 10 | No Layer Proliferation | PASS | Uses only Domain, Application, Adapter — no new layers |
| 11 | Backend Only | PASS | No frontend code |
| 12 | JSON Serializability | PASS | Custom serializers for kotlin.uuid.Uuid and java.time.Month; all objects round-trip through JSON |
| 13 | RFC 7807 Error Responses | N/A | No REST endpoints in this feature (command side only) |

**Gate result**: PASS — no violations.

## Project Structure

### Documentation (this feature)

```text
specs/001-account-domain/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
src/main/kotlin/com/walletaccountant/
├── domain/
│   ├── account/
│   │   ├── Account.kt                         # Event-sourced entity (aggregate)
│   │   ├── AccountId.kt                        # Aggregate ID value object
│   │   ├── BankName.kt                         # Enum
│   │   ├── AccountType.kt                      # Enum
│   │   ├── command/
│   │   │   └── RegisterNewAccountCommand.kt    # Command data class
│   │   └── event/
│   │       └── NewAccountRegisteredEvent.kt    # Event data class
│   └── shared/
│       ├── Money.kt                            # Value object (BigDecimal, scale 2)
│       ├── Currency.kt                         # Enum
│       └── Date.kt                             # Value object (LocalDate wrapper)
├── application/
│   ├── interceptor/
│   │   ├── HasAggregateId.kt                   # Interface for ID regeneration
│   │   └── AggregateCreationRetryInterceptor.kt
│   └── configuration/
│       └── JacksonConfiguration.kt             # Custom serializers
└── adapter/
    └── (empty — no adapters needed for this feature)

src/test/kotlin/com/walletaccountant/
├── domain/
│   ├── account/
│   │   └── AccountAggregateTest.kt             # Aggregate fixture tests
│   └── shared/
│       ├── MoneyTest.kt                        # Money precision tests
│       └── DateTest.kt                         # Date value object tests
├── application/
│   └── interceptor/
│       └── AggregateCreationRetryInterceptorTest.kt
└── serialization/
    └── JacksonSerializationTest.kt             # Round-trip JSON tests for all types
```

**Structure Decision**: Standard hexagonal architecture with Domain/Application/Adapter layers as defined in the constitution. Account aggregate gets its own folder under `domain/`. Shared value objects (Money, Currency, Date) live in `domain/shared/` since they will be reused across future aggregates. Commands and events are sub-packages under the aggregate folder.

## Complexity Tracking

No violations — table not needed.
