# Context Map

Wallet Accountant tracks where a Household's money goes and how that compares to what it planned. It is one Go binary made of four bounded contexts plus a shared kernel (see ADR-0009).

## Contexts

- [Shared kernel](./internal/sharedkernel/CONTEXT.md): currency and exchange-rate language, and Match, which every context uses
- [Households](./internal/households/CONTEXT.md): Users, their admission through Invitations, Households and membership; the ownership and privacy boundary
- [Ledger](./internal/ledger/CONTEXT.md): Accounts, Categories, Transactions and Postings; balances and Month close
- [Ingestion](./internal/ingestion/CONTEXT.md): getting transactions in from Connections, categorising them with Rules, and Matching or Accepting them
- [Planning](./internal/planning/CONTEXT.md): Budgets, Scheduled transactions and Forecasts

## Relationships

- **Households → all contexts**: every other context references Households and Users by ID only. Everything is scoped to one Household.
- **Ingestion → Ledger (immediate)**: Accepting or Matching an Imported transaction creates or confirms a Ledger Transaction in the same database transaction. An Account's first Statement import records its Opening balance in the same way, and voiding that import reverses it. Every import, Keep, Discard and Void reads the Account's Open month first, to ignore rows in closed months and to refuse changes outside the Open month (ADR-0024). Ingestion references Accounts and Categories (in Rules) by ID.
- **Ledger → Ingestion (immediate)**: before a Month close, Ledger asks Ingestion whether every Imported transaction for that Account and month is Matched or Accepted, and whether every relevant Balance check passed. Reversing an Accepted Transaction returns its Imported transaction to pending in the same database transaction.
- **Ledger → Planning (deferred)**: Ledger events update Budget actuals and Forecasts and trigger Match suggestions for Scheduled transaction occurrences. Planning references Accounts and Categories by ID.

What happens immediately and what happens later, command by command, is listed in [docs/architecture/consistency.md](./docs/architecture/consistency.md).
