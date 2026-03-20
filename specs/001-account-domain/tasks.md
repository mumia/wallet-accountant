# Tasks: Account Domain

**Input**: Design documents from `/specs/001-account-domain/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Tests**: Required per constitution principle #4 (Test-Driven Development). Tests written before implementation (Red-Green-Refactor).

**Organization**: Single user story (P1: Register a New Account). Tasks grouped as Setup → Foundational → US1 → Polish.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Gradle project initialization and Spring Boot bootstrap

- [x] T001 Create Gradle build configuration with Kotlin DSL, AF5 BOM 5.0.3, Spring Boot 3.5.3, axon-spring-boot-starter 5.0.3, Jackson, JUnit 5 in `build.gradle.kts`
- [x] T002 Create Gradle settings file in `settings.gradle.kts`
- [x] T003 Create Spring Boot application entry point in `src/main/kotlin/com/walletaccountant/WalletAccountantApplication.kt`
- [x] T004 Create application configuration in `src/main/resources/application.yml` with Axon Server and MongoDB connection defaults
- [x] T005 Update `.gitignore` for Kotlin/Gradle project (replace Go-centric entries with build/, .gradle/, out/, etc.)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared value objects, serialization, and cross-cutting infrastructure that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T006 [P] Write Money value object tests (construction with scale 2, equality, edge cases for >2 decimal places) in `src/test/kotlin/com/walletaccountant/domain/shared/MoneyTest.kt`
- [x] T007 [P] Write Date value object tests (construction, month/year accessors, ISO serialization format) in `src/test/kotlin/com/walletaccountant/domain/shared/DateTest.kt`
- [x] T008 [P] Write Jackson round-trip serialization tests for all value objects (AccountId, Money, Currency, BankName, AccountType, Date, Month, Year) in `src/test/kotlin/com/walletaccountant/serialization/JacksonSerializationTest.kt`

### Implementation for Foundational

- [x] T009 [P] Implement Currency enum (EUR, USD, CHF with display names) in `src/main/kotlin/com/walletaccountant/domain/shared/Currency.kt`
- [x] T010 [P] Implement Money value object (BigDecimal with scale 2, enforced at construction) in `src/main/kotlin/com/walletaccountant/domain/shared/Money.kt`
- [x] T011 [P] Implement Date value object (LocalDate wrapper with month/year accessors) in `src/main/kotlin/com/walletaccountant/domain/shared/Date.kt`
- [x] T012 [P] Implement AccountId value object (kotlin.uuid.Uuid wrapper with @JsonValue/@JsonCreator) in `src/main/kotlin/com/walletaccountant/domain/account/AccountId.kt`
- [x] T013 [P] Implement HasAggregateId interface for generic ID regeneration in `src/main/kotlin/com/walletaccountant/application/interceptor/HasAggregateId.kt`
- [x] T014 Implement Jackson configuration with custom serializers for kotlin.uuid.Uuid and java.time.Month in `src/main/kotlin/com/walletaccountant/application/configuration/JacksonConfiguration.kt`
- [x] T015 Verify all foundational tests pass (T006, T007, T008 go green)

**Checkpoint**: Shared value objects and serialization working — user story implementation can begin

---

## Phase 3: User Story 1 — Register a New Account (Priority: P1) 🎯 MVP

**Goal**: A RegisterNewAccount command with valid data produces a NewAccountRegistered event. Duplicate IDs are handled gracefully by the retry interceptor.

**Independent Test**: Submit a RegisterNewAccount command via Axon test fixture and verify a NewAccountRegistered event is emitted with all provided fields. Submit with duplicate ID and verify interceptor retries.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T016 [P] [US1] Write Account aggregate tests (register with all fields, register with optional notes, verify event fields including derived month/year) using Axon test fixture in `src/test/kotlin/com/walletaccountant/domain/account/AccountAggregateTest.kt`
- [x] T017 [P] [US1] Write AggregateCreationRetryInterceptor tests (successful passthrough, retry on duplicate ID, non-creation command passthrough) in `src/test/kotlin/com/walletaccountant/application/interceptor/AggregateCreationRetryInterceptorTest.kt`

### Implementation for User Story 1

- [x] T018 [P] [US1] Implement BankName enum (BCP, N26, WISE with display names) in `src/main/kotlin/com/walletaccountant/domain/account/BankName.kt`
- [x] T019 [P] [US1] Implement AccountType enum (CHECKING, SAVINGS with display names) in `src/main/kotlin/com/walletaccountant/domain/account/AccountType.kt`
- [x] T020 [P] [US1] Implement RegisterNewAccountCommand data class with @TargetEntityId on accountId, implementing HasAggregateId, in `src/main/kotlin/com/walletaccountant/domain/account/command/RegisterNewAccountCommand.kt`
- [x] T021 [P] [US1] Implement NewAccountRegisteredEvent data class with @EventTag on accountId, including derived month/year fields, in `src/main/kotlin/com/walletaccountant/domain/account/event/NewAccountRegisteredEvent.kt`
- [x] T022 [US1] Implement Account event-sourced entity with @EventSourcedEntity, static @CommandHandler (companion object + EventAppender), and @EventSourcingHandler in `src/main/kotlin/com/walletaccountant/domain/account/Account.kt`
- [x] T023 [US1] Implement AggregateCreationRetryInterceptor (MessageHandlerInterceptor<CommandMessage>, catches duplicate ID, regenerates via HasAggregateId, retries) in `src/main/kotlin/com/walletaccountant/application/interceptor/AggregateCreationRetryInterceptor.kt`
- [x] T024 [US1] Verify all US1 tests pass (T016, T017 go green)

**Checkpoint**: RegisterNewAccount command works end-to-end, duplicate ID handling verified — MVP complete

---

## Phase 4: Polish & Cross-Cutting Concerns

**Purpose**: Serialization verification and final validation

- [x] T025 Verify all Jackson serialization tests pass (T008 green) — confirm round-trip for commands, events, and all nested value objects
- [x] T026 Run full test suite (`./gradlew test`) and verify all tests pass
- [x] T027 Run quickstart.md validation — verify build and test commands work as documented

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup (Phase 1) — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational (Phase 2)
- **Polish (Phase 4)**: Depends on User Story 1 (Phase 3)

### Within Each Phase

- Tests (T006–T008, T016–T017) MUST be written and FAIL before implementation
- Value objects (T009–T012) before command/event (T020–T021)
- Command/event before aggregate (T022)
- Aggregate before interceptor tests can pass (T017 depends on T022 for fixture setup)

### Parallel Opportunities

**Phase 2 — Foundational tests (all parallel)**:
- T006 (MoneyTest) ‖ T007 (DateTest) ‖ T008 (JacksonSerializationTest)

**Phase 2 — Foundational implementation (all parallel)**:
- T009 (Currency) ‖ T010 (Money) ‖ T011 (Date) ‖ T012 (AccountId) ‖ T013 (HasAggregateId)

**Phase 3 — US1 tests (parallel)**:
- T016 (AccountAggregateTest) ‖ T017 (AggregateCreationRetryInterceptorTest)

**Phase 3 — US1 implementation (parallel group 1)**:
- T018 (BankName) ‖ T019 (AccountType) ‖ T020 (RegisterNewAccountCommand) ‖ T021 (NewAccountRegisteredEvent)

---

## Parallel Example: Foundational Phase

```bash
# Launch all foundational tests together (write first, expect failures):
Task: "Write Money value object tests in src/test/kotlin/.../MoneyTest.kt"
Task: "Write Date value object tests in src/test/kotlin/.../DateTest.kt"
Task: "Write Jackson serialization tests in src/test/kotlin/.../JacksonSerializationTest.kt"

# Then launch all value object implementations together:
Task: "Implement Currency enum in src/main/kotlin/.../Currency.kt"
Task: "Implement Money value object in src/main/kotlin/.../Money.kt"
Task: "Implement Date value object in src/main/kotlin/.../Date.kt"
Task: "Implement AccountId value object in src/main/kotlin/.../AccountId.kt"
Task: "Implement HasAggregateId interface in src/main/kotlin/.../HasAggregateId.kt"
```

## Parallel Example: User Story 1

```bash
# Launch US1 tests together (write first, expect failures):
Task: "Write Account aggregate tests in src/test/kotlin/.../AccountAggregateTest.kt"
Task: "Write AggregateCreationRetryInterceptor tests in src/test/kotlin/.../AggregateCreationRetryInterceptorTest.kt"

# Then launch US1 enums + command/event together:
Task: "Implement BankName enum in src/main/kotlin/.../BankName.kt"
Task: "Implement AccountType enum in src/main/kotlin/.../AccountType.kt"
Task: "Implement RegisterNewAccountCommand in src/main/kotlin/.../RegisterNewAccountCommand.kt"
Task: "Implement NewAccountRegisteredEvent in src/main/kotlin/.../NewAccountRegisteredEvent.kt"

# Then sequentially:
Task: "Implement Account entity in src/main/kotlin/.../Account.kt"
Task: "Implement AggregateCreationRetryInterceptor in src/main/kotlin/.../AggregateCreationRetryInterceptor.kt"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T005)
2. Complete Phase 2: Foundational (T006–T015) — tests first, then implementation
3. Complete Phase 3: User Story 1 (T016–T024) — tests first, then implementation
4. **STOP and VALIDATE**: Run `./gradlew test` — all tests green
5. Complete Phase 4: Polish (T025–T027)

### Single User Story — No Incremental Delivery Needed

This feature has only one user story (P1). The MVP IS the complete feature. After Phase 3, the Account domain is fully functional with:
- Account aggregate accepting RegisterNewAccount commands
- NewAccountRegistered events with all fields including derived month/year
- All value objects with constraints enforced
- Duplicate ID handling via retry interceptor
- Full JSON serialization support
- Comprehensive test coverage

---

## Notes

- [P] tasks = different files, no dependencies
- [US1] label maps task to User Story 1 (Register a New Account)
- Constitution principle #4 (TDD) requires tests before implementation
- Commit after each task or logical group
- All objects must be immutable (constitution principle #9)
- All command/event payloads must round-trip through JSON (constitution principle #12)
