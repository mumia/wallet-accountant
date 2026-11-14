# Consistency map

What each command changes immediately (in the same database transaction) and what follows later through domain events. The rule behind it is ADR-0013: immediate if a User would notice a delay or an inconsistency, deferred otherwise.

Every spec that adds or changes a command adds or updates its row here. Code review checks the code against this table.

| Command | Context | Immediate (same transaction) | Deferred (via events) |
|---|---|---|---|
| Accept imported transaction | Ingestion → Ledger | Imported transaction marked Accepted; Ledger Transaction recorded; touched Accounts' balances updated | Category totals, Budget actuals, Forecasts, Match suggestions |
