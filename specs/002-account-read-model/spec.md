# Feature Specification: Account Domain Read Model

**Feature Branch**: `002-account-read-model`
**Created**: 2026-03-21
**Status**: Draft
**Input**: User description: "Consume the NewAccountRegisteredEvent and add a record to an account collection in MongoDB. Make sure that the aggregate id can be used to query the item. Events are consumed in projections. Create queries to read the model. Read all accounts and a specific account by aggregate id."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View a Specific Account by ID (Priority: P1)

When an account has been registered, a user can retrieve that account's details by its aggregate ID. The system consumes the registration event, persists a read model record, and responds to queries for that specific account.

**Why this priority**: Querying a single account by its unique identifier is the most fundamental read operation — it proves the full event-to-read-model pipeline works end-to-end.

**Independent Test**: Can be fully tested by registering an account (emitting a NewAccountRegisteredEvent), then querying by aggregate ID and verifying all fields are returned correctly.

**Acceptance Scenarios**:

1. **Given** a NewAccountRegisteredEvent has been processed, **When** a query requests the account by its aggregate ID, **Then** the system returns the account with all its fields (bank name, account name, type, starting balance, currency, starting date, month, year, and notes).
2. **Given** no account exists for a given aggregate ID, **When** a query requests that account, **Then** the system indicates the account was not found.

---

### User Story 2 - View All Accounts (Priority: P2)

A user can retrieve a list of all registered accounts. This enables overview screens showing all accounts in the system.

**Why this priority**: Listing all accounts is the second most common read operation, required for any dashboard or account selection UI.

**Independent Test**: Can be fully tested by registering multiple accounts and querying for all accounts, verifying each appears in the result.

**Acceptance Scenarios**:

1. **Given** multiple NewAccountRegisteredEvents have been processed, **When** a query requests all accounts, **Then** the system returns a list containing all registered accounts with their full details.
2. **Given** no accounts have been registered, **When** a query requests all accounts, **Then** the system returns an empty list.

---

### Edge Cases

- What happens when the same NewAccountRegisteredEvent is replayed (idempotency)? The projection must handle re-processing without creating duplicate records.
- What happens when the event contains a null notes field? The read model must store and return null for optional fields.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST consume NewAccountRegisteredEvent in a projection and persist a read model record to the account collection.
- **FR-002**: System MUST store the aggregate ID in a way that supports efficient querying by aggregate ID.
- **FR-003**: System MUST support a query that retrieves a single account by its aggregate ID, returning all account fields.
- **FR-004**: System MUST support a query that retrieves all accounts, returning all account fields for each.
- **FR-005**: System MUST handle event replay idempotently — re-processing the same event must not create duplicate records.
- **FR-006**: System MUST preserve all event data in the read model: aggregate ID, bank name, account name, account type, starting balance, currency, starting date, month, year, and notes.

### Key Entities

- **Account Read Model**: A denormalized view of an account built from event data. Contains: aggregate ID (unique identifier), bank name, account name, account type, starting balance, currency, starting date, month, year, and optional notes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After a NewAccountRegisteredEvent is processed, the account is immediately queryable by its aggregate ID with all fields intact.
- **SC-002**: Querying all accounts returns every registered account with no missing records.
- **SC-003**: Querying a non-existent aggregate ID returns a clear "not found" indication rather than an error.
- **SC-004**: Replaying the same event does not produce duplicate records in the read model.

## Assumptions

- The projection processes events in-order from the Axon event store; no out-of-order delivery handling is needed.
- No pagination is required for the "all accounts" query at this stage — the account list is expected to remain small.
- No filtering, sorting, or search capabilities are needed beyond the two specified queries (by ID and all).
