# Data Model: Account Domain

**Feature**: 001-account-domain
**Date**: 2026-03-20

## Entities

### Account (Aggregate / Event-Sourced Entity)

The aggregate root for the account domain. Tracks financial account state through event sourcing.

| Field | Type | Description |
|-------|------|-------------|
| accountId | AccountId | Unique identifier |
| bankName | BankName | Financial institution |
| name | String | User-given account name |
| accountType | AccountType | CHECKING or SAVINGS |
| startingBalance | Money | Initial balance |
| currency | Currency | Account denomination |
| startingDate | Date | When the account starts |
| month | Month | Month derived from startingDate |
| year | Year | Year derived from startingDate |
| notes | String? | Optional user notes |

**State transitions**:
- `(empty)` → RegisterNewAccount command → NewAccountRegistered event → Account with all fields populated

**Invariants**:
- AccountId must be unique (enforced by event store + retry interceptor)
- All required fields must be present at creation
- Money precision must be exactly 2 decimal places

## Value Objects

### AccountId

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Globally unique identifier |

- Wraps `kotlin.uuid.Uuid`
- JSON serialization: string representation of UUID
- Implements shared aggregate ID pattern (interface-based for retry interceptor)

### BankName (Enum)

| Value | Display Name |
|-------|-------------|
| BCP | Millennium BCP |
| N26 | N26 |
| WISE | Wise |

### AccountType (Enum)

| Value | Display Name |
|-------|-------------|
| CHECKING | Checking |
| SAVINGS | Savings |

### Currency (Enum)

| Value | Display Name |
|-------|-------------|
| EUR | Euro |
| USD | US Dollar |
| CHF | Swiss Franc |

### Money

| Field | Type | Description |
|-------|------|-------------|
| amount | BigDecimal | Monetary value, scale 2 |

- Exactly 2 decimal places (enforced at construction)
- JSON serialization: numeric value

### Date

| Field | Type | Description |
|-------|------|-------------|
| value | LocalDate | Day-precision date |

- Serialized as ISO-8601 string (YYYY-MM-DD)
- Provides accessors for `month` (java.time.Month) and `year` (java.time.Year)

## Commands

### RegisterNewAccount

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| accountId | AccountId | Yes | Target aggregate ID |
| bankName | BankName | Yes | Financial institution |
| name | String | Yes | Account name |
| accountType | AccountType | Yes | Account category |
| startingBalance | Money | Yes | Initial balance |
| currency | Currency | Yes | Denomination |
| startingDate | Date | Yes | Account start date |
| notes | String? | No | Optional notes |

- `accountId` annotated with `@TargetEntityId`
- Immutable data class
- Implements `HasAggregateId` interface for retry interceptor

## Events

### NewAccountRegistered

| Field | Type | Description |
|-------|------|-------------|
| accountId | AccountId | Aggregate ID |
| bankName | BankName | Financial institution |
| name | String | Account name |
| accountType | AccountType | Account category |
| startingBalance | Money | Initial balance |
| currency | Currency | Denomination |
| startingDate | Date | Account start date |
| month | Month | Derived from startingDate |
| year | Year | Derived from startingDate |
| notes | String? | Optional notes |

- `accountId` annotated with `@EventTag(key = "accountId")`
- Immutable data class
- Contains derived fields (month, year) computed at command handling time

## Relationships

```
Account (aggregate)
├── has AccountId (identity)
├── has BankName (enum)
├── has AccountType (enum)
├── has Money (value object)
│   └── amount: BigDecimal (scale 2)
├── has Currency (enum)
├── has Date (value object)
│   └── derives Month, Year
└── has notes: String? (optional)
```
