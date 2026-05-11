---
type: prd
id: PRD-001
title: Account aggregate — register a new account
status: draft
author: Miguel Manso
stakeholders: []
created_at: 2026-05-11T19:53:20Z
references:
  adrs: [ADR-001, ADR-002, ADR-004]
  invariants: [INV-002, INV-003]
---

# PRD-001: Account aggregate — register a new account

**Status:** draft
**Date:** 2026-05-11
**Author:** Miguel Manso

---

## Problem

wallet-accountant is a multi-tenant personal accounts manager (per `docs/project-context.md`). A user — i.e., a tenant — needs to be able to register the bank accounts they want to track so the system can record transactions against them, categorise spending, and produce monthly views.

Today there is no domain code; this PRD captures the foundational vocabulary (Account aggregate, supporting value objects, the first command/event pair) on which every later transaction-tracking feature will build. Without it, transaction ingestion, categorisation, budgeting, and reporting have nothing to attach to.

## Users

A **tenant** of wallet-accountant — a household, family, or shared-finance group of one or more users who share a single set of accounts and transactions. Per [ADR-002](../../architecture/decisions/ADR-002-multi-tenant-isolation.md), each request carries a `tenantId` resolved from a JWT `tid` claim, and every command / event / read model is tenant-scoped. The tenant typically:

- Has 1..N users (typical N is small — couple, family of 3–5) who all share the same accounts.
- Holds 1–5 accounts across one or more banks (Millennium BCP, N26, Wise) and account types (Checking, Savings).
- Wants to see balances and transactions per account, organised by month.
- Operates entirely in a single currency per account, but may hold accounts in different currencies (EUR, USD, CHF).

**User attribution within a tenant**: which user (within the tenant) issued a command is captured as Axon event metadata (the JWT `sub` claim, propagated via a `MessageDispatchInterceptor` on the command bus). It is NOT a first-class domain field on commands or events — the domain shape stays tenant-scoped. See Technical Notes and Resolved Decisions.

## Goals

- Define the **Account** aggregate as the unit of consistency for one bank account belonging to one tenant.
- Establish the foundational value objects (`AccountId`, `AggregateId`, `BankName`, `AccountType`, `Money`, `Currency`, `Date`) and enumerations the rest of the domain will reuse.
- Capture the **`RegisterAccount`** command and the resulting **`AccountRegistered`** domain event — the first command/event pair in the system.
- Honor [ADR-002](../../architecture/decisions/ADR-002-multi-tenant-isolation.md) (every command/event/aggregate carries `tenantId`, validated in the `@CommandHandler` before any event is emitted) and [INV-002](../../architecture/invariants/INV-002-domain-no-framework-dependencies.md) (domain layer is framework-free).
- Produce types that read naturally in Kotlin (`data class`, `value class`, sealed type as appropriate) and serialise predictably over the wire.

## Non-Goals

- **Transactions, ledger entries, balance computation beyond the starting balance.** Those are separate PRDs.
- **Account closure / deletion / re-opening.** Out of scope for v1.
- **Editing account fields after registration** (renaming, changing notes, switching bank). Future PRD.
- **Multi-currency accounts within a single account** — each account has exactly one currency.
- **Cross-tenant shared accounts** (one account belonging to multiple tenants) — single-tenant ownership only. Within a single tenant, however, multiple users DO share the tenant's accounts (that is the point of multi-user tenants); per-user account ownership *inside* a tenant is explicitly NOT a v1 concept.
- **Importing existing transactions retroactively** — registration sets a `startingBalance` as of `startingBalanceDate`; everything before that date is out of system scope.
- **REST endpoint / OpenAPI spec / controller code** — this PRD defines the domain shape; the API surface is a downstream concern that will follow [ADR-006](../../architecture/decisions/ADR-006-api-contract-foundation.md) when speced.

## Requirements

### Must Have

- **FR-001** *[MUST]* — An `Account` aggregate exists in the domain layer at `**/domain/account/Account.kt`. Its identity is an `AccountId`. It stores at minimum: `tenantId`, `bankName`, `name`, `accountType`, `currency`, `startingBalance`, `startingBalanceDate`, `notes`, and the **active monthly ledger pointer** as separate fields `activeMonth: java.time.Month` and `activeYear: java.time.Year`. These two fields identify which monthly ledger is currently open for writing transactions; on registration they are initialised from `startingBalanceDate` (month + year of that date). They are advanced via future commands (e.g., `CloseMonth` / `OpenMonth`) that are out of scope for this PRD.
- **FR-002** *[MUST]* — An `AggregateId` abstraction exists as a domain **interface** at `**/domain/shared/AggregateId.kt` that aggregate-bound identifiers (`AccountId`, future `TransactionId`, `BudgetId`, etc.) implement. It carries a single `value: UUID` property. Implementations are Kotlin `@JvmInline value class`es. Domain types only — no Spring, Axon, or Jackson references (per INV-002). **`TenantId` is NOT an `AggregateId`** — see FR-003a.
- **FR-003** *[MUST]* — `AccountId` is a concrete `AggregateId` (`@JvmInline value class AccountId(override val value: UUID) : AggregateId`) under `**/domain/account/`. It is the value passed to Axon's `@TargetAggregateIdentifier` on every Account-bound command.
- **FR-003a** *[MUST]* — `TenantId` is a standalone domain value object at `**/domain/shared/TenantId.kt` (`@JvmInline value class TenantId(val value: UUID)`). It is **a peer of `AggregateId`, not a subtype** — tenants are not event-sourced aggregates in this project; tenant identity originates from a Zitadel-issued JWT `tid` claim (per [ADR-005](../../architecture/decisions/ADR-005-oauth-zitadel-local-jwt.md)) and flows in via the `TenantContext` bean (per [ADR-002](../../architecture/decisions/ADR-002-multi-tenant-isolation.md)). NEVER make `TenantId` implement `AggregateId`; the type system MUST keep "tenant key" and "aggregate identity" distinct.
- **FR-004** *[MUST]* — A `RegisterAccount` command exists at `**/domain/account/commands/RegisterAccount.kt`. Fields: `tenantId: TenantId` (per ADR-002), `accountId: AccountId`, `bankName: BankName`, `name: String`, `accountType: AccountType`, `startingBalance: Money`, `startingBalanceDate: Date`, `currency: Currency`, `notes: String?`. All fields except `notes` are non-null. **The `accountId` value is generated by the inbound adapter** (the web controller in `adapter/in/web/`) before dispatch — the user does NOT supply `accountId` in the HTTP request body. The web adapter returns the generated `accountId` in the response so the client can use it as an idempotency key on retries.
- **FR-005** *[MUST]* — An `AccountRegistered` domain event exists at `**/domain/account/events/AccountRegistered.kt` mirroring the `RegisterAccount` fields one-for-one. It is emitted via `AggregateLifecycle.apply(...)` from the `@CommandHandler` after validation succeeds. Per INV-003, this is the only path that writes the event.
- **FR-006** *[MUST]* — `BankName` is a domain enumeration with exactly three values: `BCP` (display: "Millennium BCP"), `N26` (display: "N26"), `WISE` (display: "Wise"). NEVER allow a free-form bank-name string at the domain layer.
- **FR-007** *[MUST]* — `AccountType` is a domain enumeration with exactly two values: `CHECKING` (display: "Checking"), `SAVINGS` (display: "Savings").
- **FR-008** *[MUST]* — `Money` is a domain value object backed by a single `BigDecimal` value. **`Money` does NOT carry a `Currency`** — currency is account-scoped and derived from `Account.currency` at every use site. `Money`'s stored scale MUST be exactly 2 decimals. Construction policy: the value is normalised via `BigDecimal.setScale(2, RoundingMode.UNNECESSARY)` — this **auto-scales-up** when the input scale is < 2 (e.g., `BigDecimal("10")` becomes `10.00`, `BigDecimal("10.5")` becomes `10.50`, both lossless) and **rejects with `ArithmeticException`** when the input scale is > 2 (e.g., `BigDecimal("10.555")` throws). Arithmetic on `Money` is currency-agnostic — addition / subtraction operates on the `BigDecimal` and preserves the 2-decimal invariant; mixing values from accounts of different currencies in a single arithmetic operation is a *caller-side* concern that the operation (e.g., a future cross-currency transfer) MUST guard by inspecting each side's `Account.currency`. `Money` itself cannot detect a currency mismatch because it has no currency.
- **FR-009** *[MUST]* — `Currency` is a domain enumeration with exactly three values: `EUR` ("Euro"), `USD` ("US Dollar"), `CHF` ("Swiss franc"). NEVER use a free-form ISO-4217 string at the domain layer.
- **FR-010** *[MUST]* — `Date` is a domain value object wrapping a `ZonedDateTime` that is strictly `ZoneOffset.UTC` and truncated to `ChronoUnit.DAYS`. Construction with a non-UTC zone or with sub-day precision throws a domain exception. JSON serialisation (when crossed at adapter boundaries) is `ISO_LOCAL_DATE` (e.g., `2026-05-11`).
- **FR-011** *[MUST]* — The `Account` aggregate's `@CommandHandler` for `RegisterAccount` validates the following before emitting any event. NEVER emit `AccountRegistered` if any validation fails:
  - the command's `tenantId` is non-null (it is set into the aggregate's stored `tenantId` on first apply);
  - `startingBalance` is non-null `Money` (per FR-008 rules — note `Money` carries no currency; the account's `currency` field is the single authoritative source);
  - `currency` is one of the valid `Currency` enum values (FR-009);
  - `name` is non-blank with `length <= 100` characters;
  - `notes` is either null OR a non-blank string with `length <= 500` characters.
- **FR-012** *[MUST]* — Re-issuing a `RegisterAccount` command against an existing `AccountId` MUST be rejected with a domain exception (`AccountAlreadyRegistered`). The aggregate's `version > 0` state is the signal.
- **FR-012a** *[MUST]* — `name` MUST be unique per `tenantId`. Registering a second account whose `name` (case-sensitive exact match) equals the `name` of any prior `AccountRegistered` event for the same `tenantId` MUST be rejected with `AccountNameAlreadyUsed` and emit no event. Enforcement uses Axon Framework 5's Dynamic Consistency Boundary (DCB) facility (per [ADR-001](../../architecture/decisions/ADR-001-axon-5-restate-division-of-labor.md)): the `RegisterAccount` command handler MUST select prior `AccountRegistered` events scoped by `tenantId` and apply the uniqueness check atomically. NEVER enforce this via a read-model lookup before dispatch — the DCB is the consistency mechanism.

### Should Have

- **FR-013** *[SHOULD]* — Display labels for enumerations (`BankName`, `AccountType`, `Currency`) live on the enum values themselves (e.g., a `displayName: String` property), so adapter layers can read them without redefining the mapping.
- **FR-014** *[SHOULD]* — `Money` provides factory functions for the common cases — `Money.zero()`, `Money.of(amount: BigDecimal)`, `Money.of(amount: String)` — but the underlying constructor enforces the 2-decimal invariant regardless of factory used. None of these factories take a `Currency` — currency is not a `Money` concern (FR-008).
- **FR-015** *[SHOULD]* — `Date` provides `Date.today()` returning the current UTC day, and `Date.of(localDate)` for parsing `ISO_LOCAL_DATE` strings.

### Won't Have (v1)

- **FR-016** — Editing account fields after registration (rename, change notes, change bank). Future PRD.
- **FR-017** — Deactivating / closing / archiving an account. Future PRD.
- **FR-018** — Currencies beyond EUR / USD / CHF. Adding one requires extending the `Currency` enum and almost certainly other read-model migrations — out of scope for v1.
- **FR-019** — Banks beyond BCP / N26 / Wise. Same constraint.
- **FR-020** — Multi-currency within a single account.

## User Stories

**P1** — **As a** user within a tenant, **I want to** register a new bank account with its bank, type, starting balance, and currency, **so that** the system has a record I can attach transactions to.

**P2** — **As a** user within a tenant, **I want** the system to reject obviously invalid input at registration (unsupported currency, non-UTC date, blank name, notes longer than 500 chars, starting balance with > 2 decimals), **so that** I don't silently corrupt my ledger.

**P3** — **As a** user within a tenant, **I want** the registration to be idempotent or at least fail loudly on retry, **so that** a network blip during the API call doesn't create a duplicate account.

## Acceptance Criteria

- [ ] **AC-001**: A `RegisterAccount` command with valid inputs (all FR-006/007/008/009/010 enum/VO rules satisfied; tenantId present; name non-blank; currency matches starting-balance currency) MUST cause the `Account` aggregate to apply exactly one `AccountRegistered` event and transition to a non-empty state with the supplied fields. — Verify: aggregate-fixture unit test using Axon Test Fixture (`AggregateTestFixture<Account>`), `expectEvents(AccountRegistered(...))`.
- [ ] **AC-002**: `Money` exposes exactly one property (`value: BigDecimal`); no `Currency` field exists on `Money`. The arch test (ArchUnit / Konsist) asserts that no class in `domain/**` named `Money` references the `Currency` type, and that `Money`'s declared properties are exactly `{value: BigDecimal}`. — Verify: structural arch test + unit test on `Money`'s public API. (Replaces the legacy currency-mismatch check, which is impossible by construction now that `Money` carries no currency.)
- [ ] **AC-003**: A second `RegisterAccount` against an already-registered `AccountId` MUST be rejected with `AccountAlreadyRegistered` and emit no event. — Verify: aggregate-fixture unit test seeding `AccountRegistered`, then dispatching `RegisterAccount` again.
- [ ] **AC-004**: A `RegisterAccount` whose `tenantId` is null or differs from the aggregate's stored `tenantId` on a re-issued command MUST be rejected — per ADR-002. — Verify: aggregate-fixture unit test + an ArchUnit / Konsist test from ADR-002's verification list confirming the field exists and is non-null.
- [ ] **AC-005**: Constructing `Money` with a `BigDecimal` of scale > 2 MUST throw a domain exception. — Verify: unit test on `Money`.
- [ ] **AC-006**: Constructing `Date` with a `ZonedDateTime` whose zone is not `ZoneOffset.UTC`, or whose precision is finer than days, MUST throw a domain exception. — Verify: unit test on `Date`.
- [ ] **AC-007**: Every command, event, and aggregate-state class in `**/domain/account/**` is a Kotlin `data class` with `val` properties only (per the existing event-sourcing guideline) and depends only on `kotlin.*`, `java.time.*`, `java.math.BigDecimal`, `java.util.UUID`, and other `**/domain/**` types (per INV-002). — Verify: ArchUnit / Konsist test in `src/test/kotlin/.../architecture/` (the test required by ADR-003).
- [ ] **AC-008**: `AccountRegistered` carries the same set of fields as `RegisterAccount` (one-for-one), is immutable, and is emitted exclusively through `AggregateLifecycle.apply(...)` from `Account`'s `@CommandHandler` — never from controller / service / projection code (per INV-003 / event-sourcing guideline). — Verify: unit test + Konsist-style architectural assertion.
- [ ] **AC-009**: A `RegisterAccount` command whose `name` (case-sensitive exact match) equals the `name` of a previously-registered account for the same `tenantId` MUST be rejected with `AccountNameAlreadyUsed` and emit no event. — Verify: DCB-scoped integration test that registers Account A, then attempts to register Account B with `tenantId == A.tenantId` and `name == A.name`, asserting the second command throws `AccountNameAlreadyUsed` and no `AccountRegistered` event for Account B is appended to the event log. A second test asserts that the same `name` IS permitted across different `tenantId`s (cross-tenant duplicates allowed).
- [ ] **AC-010**: A `RegisterAccount` command with `notes` longer than 500 characters MUST be rejected with a domain exception; with `notes = null` or `notes` length 1–500 MUST succeed. — Verify: aggregate-fixture unit test.
- [ ] **AC-011**: `TenantId` is a `@JvmInline value class` under `**/domain/shared/`. It does NOT implement `AggregateId`. A static-analysis test (Konsist / ArchUnit) asserts the type hierarchy: `TenantId` is unrelated to `AggregateId`, and no method signature accepts `TenantId` where `AggregateId` is expected (or vice versa). — Verify: Konsist / ArchUnit architectural assertion.

## Technical Notes

- **Layer**: all types in this PRD live under `**/domain/account/**` (the aggregate, command, event, `AccountId`) and `**/domain/shared/**` (cross-cutting VOs: `AggregateId` interface, `TenantId`, `Money`, `Currency`, `Date`). The shared placement aligns with the hexagonal-ddd guideline's "shared `value-objects/` folder for VOs used across multiple aggregates."
- **Axon shape**: the aggregate uses Axon 5 (`@Aggregate`, `@AggregateIdentifier`, `@CommandHandler`, `@EventSourcingHandler`, `AggregateLifecycle.apply(...)`). The Aggregate `state: AccountState` is a `data class` mutated via `copy(...)` in event-sourcing handlers (per the event-sourcing guideline). The `RegisterAccount` handler runs inside a Dynamic Consistency Boundary scoped by `tenantId` to enforce the name-uniqueness rule (FR-012a) atomically.
- **Month / Year types**: FR-001 uses `java.time.Month` (enum JAN..DEC) and `java.time.Year` directly. Both are permitted by INV-002 (`java.time.*` is on the domain allow-list). Storing them as two fields matches the brief's "Month, Year" framing; the spec MAY combine them into a `java.time.YearMonth` internally for arithmetic convenience, but the event payload exposes them separately so the read-model can index by year alone if needed.
- **AccountId generation**: `accountId` flows on the command but is not part of the HTTP request body. The web adapter (per ADR-006's controller pattern) generates `UUID.randomUUID()` at request time, wraps it in `AccountId`, dispatches the command, and returns the value in the response. Retrying the same HTTP request (e.g., after a timeout) on the *client* side with the *same* `accountId` MUST be rejected (FR-012), giving the client an idempotency signal.
- **DCB for name uniqueness (FR-012a)**: this is the project's first concrete use of Axon 5's DCB facility — worth flagging because the spec author needs to look up Axon's current DCB API. The scope of the consistency boundary is "all `AccountRegistered` events for `tenantId == X`"; the new event is appended only if the boundary's invariant (no duplicate `name`) holds. This is the architecturally correct lane per ADR-001 (rejects sagas / pre-check read-model lookups for this kind of cross-aggregate invariant).
- **REST shape is downstream**: per the Non-Goals, this PRD does not pin the HTTP request/response body. The spec follows ADR-006 (Zalando + OpenAPI 3.1 + RFC 7807) and will introduce `POST /accounts` with a `RegisterAccountRequest` DTO (no `accountId`) and a response that includes the generated `accountId`.
- **User attribution via Axon event metadata**: the `OncePerRequestFilter` introduced by ADR-005 already extracts the JWT `tid` claim into `TenantContext`. It MUST also extract the `sub` claim into a peer request-scoped bean (`UserContext` or similar). A Spring-registered `MessageDispatchInterceptor` on Axon's command bus reads both beans and adds the user identity (`sub`) to dispatched command metadata; the event metadata inherits the same. The domain shape — commands, events, aggregate state — carries `tenantId` but NEVER carries `userId`. If the project later needs fine-grained intra-tenant permissions or per-user audit views, the metadata is queryable from the event log without changing the domain types — the upgrade path is non-breaking.
- **`UserId` is NOT a domain type — layering rule**: `UserContext` is an **application-layer** bean under `application/security/` (peer to `TenantContext`, which despite holding a domain `TenantId` value is itself an application/infrastructure bean — Spring request-scoped). If the spec introduces a `UserId` value class for type safety over the raw `sub` claim string, it MUST live at `application/security/UserId.kt` — NEVER under `domain/**`. The domain layer MUST NOT reference user identity at all; user attribution is a metadata/infrastructure concern, not a domain concept. The asymmetry with `TenantId` is deliberate: `TenantId` is in the domain because every command/event payload carries it (FR-002 / FR-003a); `UserId` is not on any command/event (Resolved Decision #7), so it has no business in domain types. The arch test required by ADR-003 SHOULD assert that no class under `domain/**` references a `UserId` or `User` symbol.

## Resolved Decisions

The open questions from the initial draft have all been resolved. Recorded here so the spec author has the answers without having to re-litigate:

1. **Month/Year semantics** → **Active monthly ledger pointer.** Identifies which month is currently open for writing transactions. Initialised at registration from `startingBalanceDate`'s month + year. Advanced via future `CloseMonth` / `OpenMonth` commands (out of scope for this PRD). (FR-001)
2. **`AccountId` generation** → **Server-generated by the inbound web adapter**, not by the user. The HTTP request body has no `accountId`; the controller creates a UUID, dispatches the command, returns the value in the response. (FR-004, Technical Notes)
3. **`Money` low-scale construction** → **Auto-scale-up** via `BigDecimal.setScale(2, RoundingMode.UNNECESSARY)`. Lossless padding for scales 0 and 1; rejects scales > 2 with `ArithmeticException`. (FR-008)
4. **`notes` length cap** → **500 characters maximum** (FR-011, AC-010). Null is permitted; non-null is non-blank.
5. **`name` uniqueness** → **Unique per `tenantId`**, case-sensitive exact match. Enforced via Axon 5 DCB scanning prior `AccountRegistered` events scoped by tenant — not via read-model pre-check. Cross-tenant duplicates are permitted. (FR-012a, AC-009)
6. **`TenantId` placement and parentage** → **Standalone `@JvmInline value class TenantId(val value: UUID)` at `**/domain/shared/TenantId.kt`.** Peer of `AggregateId`, NOT a subtype — type system enforces the distinction between "tenant key" and "aggregate identity." (FR-003a, AC-011)
7. **Multiple users per tenant** → **Yes; tenants are households / families / shared-finance groups of 1..N users sharing the same accounts.** User attribution within a tenant is tracked as Axon event metadata (JWT `sub` claim, propagated via `MessageDispatchInterceptor`), NOT as a first-class field on commands or events. Cross-tenant shared accounts remain out of scope. Domain types stay tenant-scoped; per-user audit/permissions are an event-metadata concern and remain available without changing domain shape if a future PRD calls for them. (Users, Non-Goals, Technical Notes)
8. **Currency on `Money`** → **`Money` does NOT carry a `Currency`.** `Money` is a single-property value wrapper over `BigDecimal` (strict 2-decimal scale). Currency is *account-scoped*: every `Account` aggregate has a `currency: Currency` field that is the sole source of truth for the currency of every `Money` value stored against that account. Money-level cross-currency mismatch is impossible by construction; cross-currency *operations* (e.g., a future transfer between accounts of different currencies) are an operation-level concern that MUST inspect each side's `Account.currency` — `Money` itself cannot detect it. The previous "currency-match" validation on `RegisterAccount` is dropped. (FR-008, FR-014, AC-002)

---

*Written by edikt:prd — 2026-05-11*
