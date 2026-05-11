# INV-003: Axon Server is the sole source of truth for event history

**Date:** 2026-05-01
**Status:** Active

## Statement

Domain event history exists in exactly one place — Axon Server — and is written and read only through Axon Framework's event-store APIs.

## Rationale

Event sourcing's guarantees — replay, audit, projection rebuildability, temporal queries, deterministic state from offset zero — only hold when the event log is genuinely append-only and authoritative in exactly one place. Every parallel store creates a divergence point: when two stores disagree, which copy is right? Bypassing Axon's APIs to write directly to its underlying storage skips the framework's invariants around event ordering, causation chains, and idempotency, leaving Axon unable to reason about state it didn't author. The whole event-sourced architecture rests on this rule.

## Consequences of violation

- Event-log divergence — two stores disagree about what happened, replay yields different state in different environments, and the team loses the ability to answer "what is true?".
- Loss of replayability — projections cannot be rebuilt from offset zero if events live in a parallel store with different ordering or different commit visibility.
- Audit trail unreliable — regulators and downstream consumers cannot trust "the events" because there are multiple sources, each potentially incomplete.
- Workflow correctness breaks — Restate workflows that consume events lose causal-ordering guarantees the moment a parallel store reorders them.
- Disaster recovery becomes impossible to reason about — restoring "the state" means restoring two event stores in lockstep, which Axon does not coordinate, leaving an unbounded recovery window where the system is silently inconsistent.

## Implementation

Domain events emit via `AggregateLifecycle.apply(...)` inside `@CommandHandler` methods on aggregate roots. Reads happen through `EventGateway`, `QueryGateway`, or Axon's `EventStore` interface. Read-side projections subscribe via `@EventHandler` and rebuild from offset zero when needed. Snapshots, when introduced, are caches over the event stream, never authoritative. The event-sourcing guideline captures the framework-specific mechanism (`@TargetAggregateIdentifier` for routing, `@EventSourcingHandler` for state apply, DCB for cross-aggregate operations).

## Anti-patterns

- A repository class that calls `mongoTemplate.save(domainEvent)` "to keep a searchable copy" — instantly creates a parallel event log that drifts the moment Axon writes one event the parallel path doesn't see.
- Code that reads from Axon's underlying Mongo collection directly to "skip the gateway overhead" — bypasses ordering, snapshot resolution, and upcaster logic, returning events in a state Axon never intends consumers to see.
- A custom event-replay tool that reads events from a backup file and writes them to a new Postgres table as the system of record — the backup is now the source of truth, not Axon, and the rest of the system is wrong.
- Storing "important" events in both Axon and an outbox table and having downstream consumers read from the outbox — divergence is guaranteed and silent until consumers complain about missing events.
- Re-emitting an event from a projection back through `AggregateLifecycle.apply(...)` to "fix history" — never edit history; emit a new corrective event class and document the schema evolution.

## Enforcement

- **Automated (architecture test)**: an ArchUnit / Konsist test asserts that no class outside Axon's own runtime dependencies invokes `MongoTemplate.save`, `JdbcTemplate.update`, or `Connection.prepareStatement` against any collection or table whose name contains `event`, `axon`, or `domain_event`.
- **Automated (forbidden-pattern scan)**: a CI step greps `src/main/**` for `repository.save(event)`, `eventCollection.insertOne`, or any event-persistence pattern outside Axon's generated code paths and fails on match.
- **Automated (gateway-only access)**: a periodic dependency scan reports any direct query against an event collection from application or adapter code; all event reads must trace back to `EventGateway`, `QueryGateway`, or `EventStore`.
- **Manual**: PR reviewers for any change touching event flow (aggregates, projections, sagas, replays) verify the event path goes through Axon's APIs end-to-end, with no parallel persistence.

<!-- Directives for edikt governance. Populated by /edikt:invariant:compile. -->
[edikt:directives:start]: #
source_hash: f94d46b2ed22faa3f6f499e8f0d22da0e37ba037e78c9424f2849204b3304d7d
directives_hash: d542f240a3afa302cc521313eab7c621a41e46768b0d20da6bd787655cad7014
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
scope:
  - design
  - implementation
  - review
directives:
  - "Domain event history MUST be written and read exclusively via Axon Framework's APIs (`AggregateLifecycle.apply(...)`, `EventGateway`, `QueryGateway`, `EventStore`). NEVER persist domain events to any other store as the system of record, NEVER bypass these APIs to write directly to Axon's underlying storage, and NEVER read events from anywhere other than Axon's APIs. (ref: INV-003)"
reminders:
  - "Before writing a domain event → emit via `AggregateLifecycle.apply(...)`; never `mongoTemplate.save(event)`, never write to a parallel collection, never short-circuit Axon's event-store APIs (ref: INV-003)"
verification:
  - "[ ] No `repository.save(event)`, `eventCollection.insertOne`, or direct event-store writes outside Axon Framework's runtime (ref: INV-003)"
  - "[ ] All event reads route through `EventGateway`, `QueryGateway`, or `EventStore` — no direct queries against event collections / tables from application or adapter code (ref: INV-003)"
  - "[ ] No outbox / shadow event collection used as a system of record alongside Axon Server (ref: INV-003)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
