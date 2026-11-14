---
status: accepted (validated by the spike on branch prototype/spike-eventstore-ogen)
---

# Event-sourced aggregates with stored folds, and one event store table forwarded by Watermill

Events are the source of truth, but aggregates are not rebuilt from their events on every command. Each aggregate is a **Decider**: `Decide(state, command) → events` and `Evolve(state, event) → state`. A command handler loads the aggregate's stored **fold** (its current state) and its version, decides, then evolves the fold with each new event. It encodes and decodes every event before evolving, so the live fold is exactly what a replay would produce. The handler then appends the events to the **event store** and saves the new fold, conditional on the version it loaded. Event IDs, versions and timestamps are stamped once, in the handler. We chose this over classic event sourcing (load by replay) so that reads and commands stay as cheap as plain state tables, while we still keep the full history and the ability to rebuild.

## The event store

- **One permanent table**: `global_position BIGSERIAL`, `transaction_id xid8 DEFAULT pg_current_xact_id()`, the CloudEvents attributes from ADR-0015 as columns (including the stream ID, stream version, schema version, Household and actor, correlation and causation IDs) and the event data as `jsonb`, with `UNIQUE (stream_id, version)` as a backstop for the conditional save. Rows are never updated or deleted.
- **It is also the outbox.** There is no separate outbox table. Watermill's SQL subscriber reads the event store through a custom `SchemaAdapter` (select and unmarshal only, since the repository does the inserts) and uses Watermill's default PostgreSQL offsets adapter for consumer groups. Ordering is gap-safe: readers only see rows whose `transaction_id` is below `pg_snapshot_xmin(pg_current_snapshot())`. This requires PostgreSQL 13 or later.
- **Forwarding** starts in-process: Watermill router handlers in the `worker` run mode. A message broker can be added later as one more handler that publishes to it.
- **Replay** for a projection means a new or reset consumer group reading from position 0.
- **Rebuilding a fold** means reading one stream in version order through the same codec, `upcast` step and `Evolve`.

We chose one table over writing every event twice (to our event store and to Watermill's own table) because Watermill's default schema never deletes messages, so that would keep two permanent copies of history that could drift apart.

## Aggregate boundaries in the Ledger

A Transaction is an aggregate that enforces "Postings sum to zero". Each Account it touches is also an aggregate, holding its running balance per month and its Month close state. The application service runs every affected Decider in **one database transaction**, each with its own version check, and retries on a conflict. Month close therefore checks a balance that is always current. Category totals are not aggregates. They are projections, because Budgets can lag.

## Stale folds

Each stored fold records the **fold schema version** it was built with. When `Load` sees an older version, it rebuilds that fold from its stream and saves it. The `rebuild` run mode (ADR-0009) does the same in bulk after a deploy. A change to fold structure is usually handled by bumping the fold schema version and rebuilding, not by an SQL data migration.

## Reads

The API reads a single aggregate's stored fold directly, for example an Account's balance or Month close status, which is always current. Views that span aggregates are projections built asynchronously by Watermill handlers, and are listed as deferred in `docs/architecture/consistency.md`.

## Event handlers

Watermill runs select, handler and offset update in **one** subscriber transaction and passes it to the handler (`sql.TxFromContext`). A projection's writes and its consumer offset therefore commit together. The spike also showed that Watermill **commits that transaction even when a message is not acked**: a Nack retries the message inside the same transaction, and a shutdown before Ack commits whatever the handler already wrote without moving the offset. So every handler must:

- wrap each attempt in a `SAVEPOINT` and roll back to it before a Nack, and
- record `(consumer_group, event_id)` in a processed-events table in the same transaction, and skip events already recorded, so a redelivery after a shutdown changes nothing.

Gaps in `global_position` are normal (a failed insert still uses up a sequence value), so nothing may assume positions are contiguous.

## Consequences

- Event schemas are a long-lived contract: every event type carries a `schema_version`, and old events are upcast when read, never rewritten.
- The custom Watermill schema adapter is ours to maintain. The spike proved that it is gap-safe (an event committed after a slower transaction is held back until that transaction commits, then both arrive in order, exactly once) and that a new consumer group replays from position 0.
