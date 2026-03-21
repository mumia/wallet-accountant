# Feature Specification: Account Domain

**Feature Branch**: `001-account-domain`
**Created**: 2026-03-20
**Status**: Draft
**Input**: User description: "Account domain with Account aggregate, RegisterNewAccount command/event, and supporting value objects"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Register a New Account (Priority: P1)

A user provides account details — bank, account name, account type, starting balance with currency, starting date, and optional notes — and the system creates a new account that can be used to track financial transactions.

**Why this priority**: This is the foundational operation for the entire application. Without the ability to register accounts, no other financial tracking functionality can exist.

**Independent Test**: Can be fully tested by submitting a RegisterNewAccount command with valid details and verifying the account is created with the correct state. Delivers the core capability of account creation.

**Acceptance Scenarios**:

1. **Given** no account exists with the provided ID, **When** a user submits a RegisterNewAccount command with bank name "BCP", account name "Main Checking", type "CHECKING", starting balance 1000.50 EUR, and date 2026-01-15, **Then** the system creates the account and emits a NewAccountRegistered event containing all provided details.

2. **Given** no account exists with the provided ID, **When** a user submits a RegisterNewAccount command with bank name "N26", account name "Savings", type "SAVINGS", starting balance 0.00 CHF, date 2026-03-01, and notes "Emergency fund", **Then** the system creates the account and emits a NewAccountRegistered event including the notes.

3. **Given** an account already exists with the provided ID, **When** a user submits a RegisterNewAccount command with the same ID, **Then** the system rejects the command (aggregate creation retry interceptor handles duplicate ID conflicts).

---

### Edge Cases

- What happens when a starting balance has more than 2 decimal places? The system MUST reject it or round to exactly 2 decimal places.
- What happens when an invalid bank name, account type, or currency is provided? The system MUST reject the command with a clear validation error.
- What happens when required fields (bank name, account name, type, balance, currency, date) are missing? The system MUST reject the command.
- What happens when the date is in the future? The system SHOULD accept it (users may pre-register accounts).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow users to register a new account by providing: bank name, account name, account type, starting balance, currency, starting date, and optional notes.
- **FR-002**: System MUST assign a unique AccountId to each new account.
- **FR-003**: System MUST track the month and year associated with the account (derived from the starting date).
- **FR-004**: System MUST persist account creation as an event (NewAccountRegistered) in the event store.
- **FR-005**: System MUST support exactly three bank names: BCP (Millennium BCP), N26 (N26), and WISE (Wise).
- **FR-006**: System MUST support exactly two account types: CHECKING (Checking) and SAVINGS (Savings).
- **FR-007**: System MUST support exactly three currencies: EUR (Euro), USD (US Dollar), and CHF (Swiss Franc).
- **FR-008**: Money values MUST have exactly 2 decimal places of precision.
- **FR-009**: Date values MUST have day precision, use UTC timezone, and serialize in ISO date format.
- **FR-010**: System MUST handle duplicate AccountId conflicts gracefully via an aggregate creation retry interceptor.

### Key Entities

- **Account**: The aggregate root. Represents a financial account that tracks transactions. Tracks month and year. Created via RegisterNewAccount command, persisted via NewAccountRegistered event.
- **AccountId**: Unique identifier for an account. One of many aggregate IDs in the system (shared identifier pattern).
- **BankName**: Enumerated value — BCP (Millennium BCP), N26 (N26), WISE (Wise). Represents the financial institution holding the account.
- **AccountType**: Enumerated value — CHECKING (Checking), SAVINGS (Savings). Categorizes the account's purpose.
- **Money**: A monetary amount with exactly 2 decimal places. Always paired with a Currency.
- **Currency**: Enumerated value — EUR (Euro), USD (US Dollar), CHF (Swiss Franc). Identifies the denomination of a Money value.
- **Date**: A date value with day precision, stored in UTC, serialized in ISO date format (YYYY-MM-DD).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A RegisterNewAccount command with valid data produces a NewAccountRegistered event containing all provided fields.
- **SC-002**: Duplicate AccountId registration is handled gracefully without system errors (retry interceptor resolves conflicts).
- **SC-003**: All value objects (Money, Date, BankName, AccountType, Currency) correctly enforce their constraints (precision, valid values, format).
- **SC-004**: Events are successfully persisted to the event store and can reconstruct aggregate state via event sourcing.
