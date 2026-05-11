# Event Sourcing Guidelines

**Purpose:** Keep the event-sourced model honest under Axon Framework 5 — protect aggregate consistency boundaries, treat the event log as immutable history, route cross-aggregate work through Dynamic Consistency Boundaries instead of ad-hoc coordination, and keep every domain object immutable so that "the past" stays the past.

## Rationale

Event sourcing trades the cost of "current state in a row" for the value of "every fact that ever happened, replayable forever". That trade only pays off if the event log is genuinely append-only, the Axon command/event/state-apply lifecycle is followed without shortcuts, and every projection remains a pure function of the event stream. The moment any of those slips — an event quietly edited, a `@CommandHandler` that bypasses `AggregateLifecycle.apply(...)`, a projection that persists derived state it cannot rebuild — the model loses its replayability and the auditability that justified its complexity.

Axon Framework 5 gives this project the right primitives: aggregates with `@CommandHandler` and `@EventSourcingHandler` to keep command validation and state apply in lockstep; `@TargetAggregateIdentifier` to bind each command to a single aggregate instance; Axon Server as the canonical event store; the new Dynamic Consistency Boundary (DCB) facility for genuinely cross-aggregate operations; and projections that can be rebuilt from offset zero. The rules below are the contract for using those primitives correctly. Restate handles durable execution for cross-system workflows; Axon handles the event-sourced write side. Each owns its lane.

**Architectural rules — single-root aggregates, value-object immutability, framework-free domain, ports/adapters, CQS — live in `hexagonal-ddd.md`.** This guideline covers only the event-sourcing-specific mechanics layered on top of that architecture; it does not restate the architectural foundations.

## Rules

- Each aggregate MUST bind every incoming command to exactly one aggregate instance via `@TargetAggregateIdentifier` on the command. NEVER write a command that targets multiple aggregates or omits the identifier.
- Commands MUST be fully validated inside the aggregate's `@CommandHandler` before any event is emitted via `AggregateLifecycle.apply(...)`. NEVER emit a domain event for a command that has not passed all invariant checks.
- Each aggregate class MUST live in its own file alongside its `@CommandHandler` and `@EventSourcingHandler` methods, and MUST remain focused on a single business concept. Aggregates exceeding ~300 lines or accumulating handlers for unrelated invariants MUST be split.
- Aggregate state transitions MUST be applied only inside `@EventSourcingHandler` methods, building the new state via `copy(...)` on the state's `data class`. NEVER mutate aggregate state inside a `@CommandHandler`, and NEVER in-place-mutate fields inside an event-sourcing handler.
- Axon Server MUST be the single source of truth for event history. NEVER persist domain events to any other store as the system of record, and NEVER bypass Axon's event-store APIs to read or write events.
- Operations spanning multiple aggregates MUST use Axon Framework 5's Dynamic Consistency Boundary (DCB) facility. NEVER coordinate across aggregates with two-phase commits, distributed transactions, or hand-rolled compensation outside DCB.
- NEVER dispatch a command to another aggregate from within an aggregate's `@CommandHandler` or `@EventSourcingHandler`. Cross-aggregate command flow MUST go through a stateless external handler (typically a Restate workflow, per ADR-001) or a DCB-coordinated unit of work. Axon sagas (`@Saga`) MUST NOT be used for this — see ADR-001.
- Commands, domain events, queries, and read-model DTOs MUST be immutable Kotlin `data class`es with `val` properties. NEVER add `var` properties, setters, or mutable collection types (`MutableList`, `MutableSet`, `MutableMap`) to any of these types.
- Domain events MUST represent facts in the past tense (`InvoicePaid`, `OrderShipped`, `AccountCredited`) and, once published, MUST NEVER be modified, deleted, or rewritten in place. Schema evolution MUST proceed via Axon upcasters or new event types — never by editing past events.
- Read models / projections MUST be fully rebuildable by replaying the event stream from offset zero. NEVER store derived state in a projection that cannot be reconstructed from events alone, and NEVER allow read-model writes outside of the projection's event-handler code path.

## When NOT to apply

These rules do not apply to:

- **Operational tooling that reads the event store for analytics, audit, or backfill** — read-only consumers (offline reports, downstream BI exports, replay tools) MAY traverse the event store outside Axon's event-handler abstractions, provided they NEVER write to it. The "Axon Server is the single source of truth" rule still holds; only the access *path* relaxes for read-only ops.
- **Test code** under `src/test/` and `src/integrationTest/`. Test fixtures MAY construct aggregates and events through internal constructors or hand-built event lists for the purpose of verifying behavior. Production code paths MUST still go through `AggregateLifecycle.apply(...)` and `@EventSourcingHandler`.
- **Genuinely cross-bounded-context coordination** that lives outside the event-sourced model — Restate workflows, external system integrations, and inbound webhooks. These MAY orchestrate aggregate commands from outside the aggregate (which is in fact the only correct place to do so), but MUST translate inbound external messages into proper Axon commands rather than mutating aggregates directly.
- **Snapshot / cache layers introduced for performance**. Aggregate snapshots and projection materialized views MAY exist as caches, provided they remain *derivable* from the event log alone and MUST NEVER become a second source of truth — the event stream stays authoritative.

These four exceptions are the only legitimate ones. "It's faster to write the read model directly", "the aggregate just needs to peek at another aggregate", and "we'll fix the event schema by editing the bad rows" are not exceptions — each one would silently break the guarantees that make event sourcing worth its complexity in the first place.

---

*Created by edikt:guideline — 2026-05-01*

<!-- Directives for edikt governance. Populated by /edikt:guideline:compile. -->
[edikt:directives:start]: #
source_hash: c04bd04ad6f92baec71c41bd83041fff8ae32a021cf99cf13912459b842303b4
directives_hash: 1c0859499d5b2f6c91b245e71239360bcc7264406c5a7d120bb4f45e3121e48b
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/domain/**"
  - "**/application/**"
  - "**/adapter/out/readmodel/**"
scope:
  - design
  - implementation
  - review
directives:
  - "Each aggregate MUST bind every incoming command to exactly one aggregate instance via `@TargetAggregateIdentifier` on the command. NEVER write a command that targets multiple aggregates or omits the identifier. (ref: event-sourcing)"
  - "Commands MUST be fully validated inside the aggregate's `@CommandHandler` before any event is emitted via `AggregateLifecycle.apply(...)`. NEVER emit a domain event for a command that has not passed all invariant checks. (ref: event-sourcing)"
  - "Each aggregate class MUST live in its own file alongside its `@CommandHandler` and `@EventSourcingHandler` methods, and MUST remain focused on a single business concept. Aggregates exceeding ~300 lines or accumulating handlers for unrelated invariants MUST be split. (ref: event-sourcing)"
  - "Aggregate state transitions MUST be applied only inside `@EventSourcingHandler` methods, building the new state via `copy(...)` on the state's `data class`. NEVER mutate aggregate state inside a `@CommandHandler`, and NEVER in-place-mutate fields inside an event-sourcing handler. (ref: event-sourcing)"
  - "Axon Server MUST be the single source of truth for event history. NEVER persist domain events to any other store as the system of record, and NEVER bypass Axon's event-store APIs to read or write events. (ref: event-sourcing)"
  - "Operations spanning multiple aggregates MUST use Axon Framework 5's Dynamic Consistency Boundary (DCB) facility. NEVER coordinate across aggregates with two-phase commits, distributed transactions, or hand-rolled compensation outside DCB. (ref: event-sourcing)"
  - "NEVER dispatch a command to another aggregate from within an aggregate's `@CommandHandler` or `@EventSourcingHandler`. Cross-aggregate command flow MUST go through a stateless external handler (typically a Restate workflow, per ADR-001) or a DCB-coordinated unit of work. Axon sagas (`@Saga`) MUST NOT be used for this — see ADR-001. (ref: event-sourcing)"
  - "Commands, domain events, queries, and read-model DTOs MUST be immutable Kotlin `data class`es with `val` properties. NEVER add `var` properties, setters, or mutable collection types (`MutableList`, `MutableSet`, `MutableMap`) to any of these types. (ref: event-sourcing)"
  - "Domain events MUST represent facts in the past tense (`InvoicePaid`, `OrderShipped`, `AccountCredited`) and, once published, MUST NEVER be modified, deleted, or rewritten in place. Schema evolution MUST proceed via Axon upcasters or new event types — never by editing past events. (ref: event-sourcing)"
  - "Read models / projections MUST be fully rebuildable by replaying the event stream from offset zero. NEVER store derived state in a projection that cannot be reconstructed from events alone, and NEVER allow read-model writes outside of the projection's event-handler code path. (ref: event-sourcing)"
reminders:
  - "Before mutating aggregate state → emit a domain event via `AggregateLifecycle.apply(...)`; apply state changes only inside `@EventSourcingHandler`, never inside a `@CommandHandler` (ref: event-sourcing)"
  - "Before coordinating across aggregates → use Axon 5 Dynamic Consistency Boundary (DCB); never dispatch a command from inside an aggregate's `@CommandHandler` or `@EventSourcingHandler` (ref: event-sourcing)"
verification:
  - "[ ] No `var` properties, setters, or mutable collection types on commands, domain events, queries, or read-model DTOs (ref: event-sourcing)"
  - "[ ] No direct command dispatch (`CommandGateway.send`, `commandBus.dispatch`) inside `@CommandHandler` or `@EventSourcingHandler` methods (ref: event-sourcing)"
  - "[ ] No domain-event persistence outside Axon Server's event-store APIs (no `repository.save(event)`, no manual writes to a separate event collection) (ref: event-sourcing)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
