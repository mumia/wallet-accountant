# PostgreSQL for state, with a transactional outbox for domain events

All state is stored in PostgreSQL. Every change that produces a domain event writes that event to an outbox table in the same database transaction as the state change, so state and events can never disagree. Workers forward outbox events afterwards. We chose PostgreSQL over SQLite because it handles many concurrent writers (needed if the app becomes a hosted service, see ADR-0005), has exact `NUMERIC` decimals, and can enforce ledger invariants with constraints. It still runs easily self-hosted with Docker Compose.

## Status

Accepted. Partly superseded by ADR-0011, which makes events the source of truth and replaces the separate outbox table with the event store itself.
