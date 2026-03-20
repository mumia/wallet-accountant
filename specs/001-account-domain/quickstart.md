# Quickstart: Account Domain

**Feature**: 001-account-domain
**Date**: 2026-03-20

## Prerequisites

- JDK 21
- Gradle 8.13+
- Axon Server running locally (default port 8024)
- MongoDB running locally (default port 27017)

## Build & Test

```bash
# Build the project
./gradlew build

# Run all tests
./gradlew test

# Run specific test class
./gradlew test --tests "com.walletaccountant.domain.account.AccountAggregateTest"

# Run a single test
./gradlew test --tests "com.walletaccountant.domain.account.AccountAggregateTest.should register new account"
```

## Project Structure

```
src/main/kotlin/com/walletaccountant/
├── domain/
│   ├── account/                    # Account aggregate
│   │   ├── Account.kt             # Event-sourced entity
│   │   ├── AccountId.kt           # Aggregate ID value object
│   │   ├── BankName.kt            # Enum: BCP, N26, WISE
│   │   ├── AccountType.kt         # Enum: CHECKING, SAVINGS
│   │   ├── command/
│   │   │   └── RegisterNewAccountCommand.kt
│   │   └── event/
│   │       └── NewAccountRegisteredEvent.kt
│   └── shared/                     # Cross-aggregate value objects
│       ├── Money.kt
│       ├── Currency.kt
│       └── Date.kt
├── application/
│   ├── interceptor/
│   │   └── AggregateCreationRetryInterceptor.kt
│   └── port/
│       └── (driving/driven ports as needed)
└── adapter/
    └── (adapters as needed)

src/test/kotlin/com/walletaccountant/
├── domain/
│   ├── account/
│   │   └── AccountAggregateTest.kt
│   └── shared/
│       ├── MoneyTest.kt
│       ├── DateTest.kt
│       └── CurrencyTest.kt (if needed)
└── application/
    └── interceptor/
        └── AggregateCreationRetryInterceptorTest.kt
```

## Key Patterns

### Creating an Account (Command → Event)

1. Client sends `RegisterNewAccountCommand` with all required fields
2. Static `@CommandHandler` in `Account` companion object receives the command
3. Handler validates inputs and appends `NewAccountRegisteredEvent` via `EventAppender`
4. `@EventSourcingHandler` applies event to reconstruct state

### Duplicate ID Handling

1. `AggregateCreationRetryInterceptor` wraps command processing
2. If aggregate creation fails due to duplicate ID, interceptor generates a new ID and retries
3. Commands implement `HasAggregateId` interface to support ID regeneration

### Value Object Immutability

All value objects are Kotlin `data class` (or `enum class`) — immutable by design. `Money` enforces scale-2 precision at construction time.
