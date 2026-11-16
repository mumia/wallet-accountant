# Modular monolith: hexagonal bounded contexts in one binary with run modes

The backend is a single Go binary containing several bounded contexts (Households, Ledger, Ingestion and Planning, plus a shared kernel; see `CONTEXT-MAP.md`). Each bounded context follows Hexagonal Architecture (ports and adapters). The domain sits at the centre and depends on no frameworks or infrastructure. The domain, the application layer and the ingestion package define ports. Adapters (HTTP, PostgreSQL, the event store and event forwarding, bank connections) implement those ports. The binary has these run modes:

- `api` serves HTTP.
- `worker` runs background work, such as forwarding events and processing imports.
- `all` runs `api` and `worker` in one process.
- `migrate` applies the embedded schema migrations, then exits.
- `rebuild` replays event streams to regenerate stored folds and projections (ADR-0011), then exits.
- `bootstrap` issues the first Instance administrator Invitation and prints its code (ADR-0017), then exits.

No mode other than `migrate` changes the schema. Migrations are forward-only, with no down migrations, and are written as expand/contract steps so the previous release keeps working against the new schema. Docker Compose runs `migrate` as a one-shot service before `all` starts, so self-hosters don't run anything by hand.

We chose one binary because that suits self-hosting (ADR-0005). The run modes and context boundaries let the API and workers scale separately, or the contexts be split into services, if the app ever becomes a hosted service.
