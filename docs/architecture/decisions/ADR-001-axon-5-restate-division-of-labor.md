---
status: accepted
date: 2026-05-02
decision-makers: [Miguel Manso]
consulted: []
informed: []
supersedes: null
---

# ADR-001: Axon Framework 5 + Restate division of labor

## Context and Problem Statement

wallet-accountant is event-sourced (CQRS via Axon Framework, Axon Server as the event store, MongoDB read models — see [INV-003](../invariants/INV-003-axon-server-sole-event-store.md)) and multi-tenant. Two classes of work need orchestration:

1. **Cross-aggregate invariants** — e.g. moving money between two `Account` aggregates of the same tenant must atomically debit one and credit the other; tenant ownership must be enforced across both writes. The classic event-sourcing answer is a saga, but a saga is eventually-consistent compensation: it cannot prevent an inconsistent intermediate state, only react to one.
2. **Long-running external workflows** — connecting a bank via OAuth, polling a statement-import provider, retrying a webhook against a third party, scheduling a "categorise this transaction in 24h if unconfirmed" reminder. These must survive process restarts, retry idempotently, and remain observable across days or weeks.

Two questions then:

- **Which CQRS / event-sourcing framework version?** Axon 4 has been the long-standing default; Axon 5 ships the **Dynamic Consistency Boundary (DCB)** — a primitive that lets a single command operate transactionally across a *selected slice* of the event stream, not just a single aggregate.
- **Which orchestration tool for long-running external workflows?** Axon's saga support can do some of this, but it is in-process, event-driven, and was not designed for arbitrary external side effects with at-most-once semantics. Dedicated durable-execution engines (Restate, Temporal) treat external calls, retries, and timers as first-class.

We need to decide the framework versions and the **boundary between them** before writing aggregates and workflows, because the wrong split leaks orchestration logic into the domain or scatters domain logic across workflow handlers.

## Decision Drivers

- **Strong consistency where the domain demands it.** Account-to-account transfers, tenant-ownership checks across aggregates, and similar invariants must hold *atomically*, not eventually.
- **Durability and idempotency for external side effects.** OAuth flows, third-party API calls, statement imports, and timers must survive process restarts and replay deterministically without double-charging anything.
- **Single ownership per concern.** One tool owns the event log; one tool owns durable orchestration. No overlapping responsibilities.
- **Operability for a small team.** Self-hostable, Spring-Kotlin idiomatic, minimal moving parts.
- **Replaceable read side.** Read models can be rebuilt from events at any time (already locked in by INV-003).

## Considered Options

1. **Axon Framework 5 (DCB) + Restate, no sagas** — Axon owns command/event modeling, aggregate state, and cross-aggregate consistency via DCB; Restate owns *all* orchestration, including reactions to domain events. Axon sagas (`@Saga` / classes implementing `Saga`) are not used.
2. **Axon Framework 5 (DCB) + Restate + Axon sagas for in-process reactions** — same as option 1, but keep sagas as a narrow lane for purely in-domain, latency-sensitive event cascades that do not cross an external boundary.
3. **Axon Framework 4 + Axon sagas only** — Stick with Axon 4. All cross-aggregate coordination and all long-running workflows go through Axon sagas.
4. **Axon Framework 5 + Temporal** — Axon for CQRS/ES, Temporal for durable execution.
5. **EventStoreDB + custom CQRS + Restate** — Replace Axon entirely with EventStoreDB and hand-rolled CQRS plumbing; keep Restate for orchestration.
6. **Kafka + Debezium + custom orchestration** — Use Kafka as the event log, Debezium for change-data-capture into read models, hand-rolled orchestration for everything else.

## Decision Outcome

Chosen option: **Axon Framework 5 (DCB) + Restate, no sagas**, because it gives us strong cross-aggregate consistency *without* sagas where it matters (DCB), a purpose-built durable-execution runtime for *all* orchestration (Restate), and a single mental model for reacting to domain events instead of two overlapping ones.

The division of labor is:

**Axon Framework 5 owns:**
- Command and domain-event modeling (`@CommandHandler`, `@EventSourcingHandler`).
- Aggregate state and aggregate-scoped invariants.
- Cross-aggregate invariants via the **Dynamic Consistency Boundary (DCB)** facility — operations that must atomically read and write events across multiple aggregates use DCB, never sagas, never two-phase commit, never hand-rolled compensation.
- The event log (Axon Server, per [INV-003](../invariants/INV-003-axon-server-sole-event-store.md)) and projections to MongoDB read models.

**Restate owns:**
- *All* orchestration — long-running multi-step workflows, external-service interactions (third-party HTTP/RPC, OAuth flows, statement imports), durable timers, scheduled reminders, human-in-the-loop steps, retries with backoff, *and* in-process reactions to domain events.
- Durable timers, idempotent external invocations, and structured concurrency for orchestration.

**Boundary rules:**
- Restate workflows interact with the domain **only** by dispatching Axon commands through Axon's `CommandGateway` and observing domain state through Axon-backed projections / queries. Restate never writes to the event store directly.
- The boundary is **unidirectional**: Restate may invoke Axon; Axon code (aggregates, command handlers, event handlers, projections) MUST NOT call into Restate.
- **Axon sagas (`@Saga` / classes implementing `Saga`) are not used.** All event reactions go through Restate. A thin Axon→Restate stream adapter forwards domain events into Restate handlers when a workflow needs to react to them.
- Restate handlers live in the driving-adapter layer at `adapter/in/restate/**`, alongside `adapter/in/web/**`. They contain orchestration only — no domain logic.

### Consequences

**Positive:**
- Cross-aggregate domain invariants (account transfer, tenant-ownership cross-checks) get true atomicity via DCB without paying the saga complexity tax.
- External side effects get a runtime that was designed for them: durable timers, idempotent retries, deterministic replay across process restarts.
- A *single* orchestration model — Restate — for every multi-step or event-reactive concern. No "is this in-process and event-driven enough to be a saga?" decision in design or PR review.
- The unidirectional boundary keeps Axon (and the domain layer) free of orchestration-runtime imports, preserving [INV-002](../invariants/INV-002-domain-no-framework-dependencies.md).

**Negative:**
- Two runtimes to operate (Axon Server + Restate), two SDKs to keep current, two failure modes to debug.
- Axon 5 is newer than 4; less battle-testing in the wild, fewer Stack Overflow answers, and DCB is a relatively new primitive whose ergonomics will evolve.
- Restate is younger than Temporal; smaller community, smaller surface of public production case studies.
- Pure in-domain event cascades (e.g., `TransactionCategorised` → `RecalculateMonthlyBudget`) pay a small latency tax — a Restate round-trip — that an in-process saga would not. Acceptable for this domain; revisit if a sub-millisecond reaction is ever genuinely required.

**Neutral:**
- All cross-aggregate work either goes through DCB (consistency-required) or Restate (orchestration-required). Sagas (`@Saga`) are not used.
- Restate handlers add a new top-level adapter directory (`adapter/in/restate/`) parallel to `adapter/in/web/`, already accounted for in the project layout.
- A thin Axon→Restate stream adapter is required so Restate workflows can react to domain events. One-time plumbing, not a recurring cost.

## Pros and Cons of the Options

### Axon Framework 5 (DCB) + Restate, no sagas (chosen)

- DCB removes the need to express cross-aggregate invariants as sagas — strong consistency where the domain demands it.
- Restate gives durable execution as a first-class primitive for external side effects: idempotent retries, deterministic replay, durable timers.
- Single orchestration model. No saga-vs-Restate decision gate in design or PR review.
- Tight directives: a flat ban on `@Saga` is trivially enforceable; the narrow-lane carve-out is not.
- Requires a small Axon→Restate stream adapter to forward domain events into Restate when a workflow must react to them.
- Both runtimes are relatively young (Axon 5 in particular), reducing the pool of accumulated production wisdom.

### Axon Framework 5 (DCB) + Restate + Axon sagas for in-process reactions

- Saves the latency of a Restate round-trip on pure in-domain event cascades.
- Sagas inherit Axon's tracking-token ordering and at-least-once-with-idempotency for free; Restate equivalents need a small stream adapter to match.
- Two orchestration models to learn, document, and gate in PR review ("is this saga-shaped or Restate-shaped?").
- Carve-out enforcement is fragile: a saga that "just temporarily" calls an external service violates the rule but is not statically obvious from a quick look at the class.
- For wallet-accountant's domain, no realistic reaction is sub-millisecond-sensitive enough to need the saga lane.
- Rejected: the latency saving is not load-bearing for this project; the cost is two overlapping orchestration models and a fragile carve-out rule.

### Axon Framework 4 + Axon sagas only

- Single framework, single mental model, broadest community knowledge.
- Sagas are eventually consistent — they cannot enforce a cross-aggregate *invariant*, only react to a violation and try to compensate. For account transfers and tenant-ownership cross-checks, that is the wrong consistency model.
- Sagas are a poor fit for external orchestration: they have no durable timer primitive on par with Restate's, no first-class idempotent external-call abstraction, and recovery is tied to event replay rather than orchestration replay.
- No DCB — every cross-aggregate operation pushed into either a saga or hand-rolled coordination outside the framework.
- Rejected: the DCB facility in Axon 5 is precisely the feature this project was waiting for, and saga-only orchestration of external systems duplicates what Restate gives us natively.

### Axon Framework 5 + Temporal

- Temporal is mature, large community, large public production footprint.
- Heavier operational footprint than Restate (Cassandra/SQL backend, history service, multiple worker types).
- Programming model is workflow-class-as-state-machine; Restate's handler-style, request/response durable functions slot more naturally into Spring/Kotlin and our existing adapter layout.
- Rejected: Restate's lighter footprint and more idiomatic Spring/Kotlin handler model fit this project's small-team operability driver better; Temporal's extra surface area is not justified by needs we have today.

### EventStoreDB + custom CQRS + Restate

- EventStoreDB is a strong, dedicated event store.
- Replacing Axon means rebuilding command bus, command handler dispatch, aggregate snapshotting, event upcasting, projection runtime, and DCB-equivalent semantics ourselves — undifferentiated work measured in months, then maintained forever.
- Loses DCB as an off-the-shelf primitive.
- Rejected: the cost of rebuilding the CQRS framework dwarfs the benefit of swapping the event store, and we lose the very feature (DCB) that motivated the Axon 5 choice.

### Kafka + Debezium + custom orchestration

- Kafka is a log, not an event store with aggregate semantics; aggregate boundaries, optimistic concurrency, snapshots, and replay-from-offset-zero would all be hand-built.
- Debezium is for change-data-capture, not domain event sourcing — modeling domain events as DB row changes inverts the dependency we want.
- No DCB-equivalent, no durable execution.
- Rejected: wrong tool for every layer — wrong event-sourcing primitive, wrong projection mechanism, no orchestration story.

## Confirmation

How we will know this decision is being followed:

- **Architecture test (ArchUnit / Konsist) — Restate isolation**: a test asserts that no class outside `adapter/in/restate/**` imports from `dev.restate.*`, and that no class under `domain/**` or `application/**` imports from `dev.restate.*` or from any Axon `commandbus` / `eventbus` *implementation* package (only `org.axonframework.*` interface gateways are allowed at the application boundary).
- **Architecture test — no sagas**: a test asserts that no class in `src/main/**` is annotated with `@Saga` and no class implements Axon's `Saga` interface. The annotation and interface are statically absent from production code.
- **Architecture test — boundary direction**: a test asserts no Axon component (`@Aggregate`, `@CommandHandler`, `@EventHandler`, `@EventSourcingHandler`, projection classes) imports from `dev.restate.*` — the Axon → Restate direction is forbidden.
- **Build pin**: `gradle/libs.versions.toml` pins `axonframework` to a `5.x` line; CI fails if a 4.x coordinate is reintroduced.
- **Manual review**: PRs introducing a new cross-aggregate operation must show in the description whether the operation uses DCB (consistency-required) or a Restate workflow (orchestration-required). PRs that introduce a saga (`@Saga` annotation or `Saga` interface implementation) MUST be rejected on sight — the codebase has no saga lane.

## More Information

- [INV-002 — Domain has no framework dependencies](../invariants/INV-002-domain-no-framework-dependencies.md)
- [INV-003 — Axon Server is the sole source of truth for event history](../invariants/INV-003-axon-server-sole-event-store.md)
- Project context: `docs/project-context.md`
- Axon Framework 5 Dynamic Consistency Boundary documentation
- Restate documentation: https://docs.restate.dev

<!-- Directives for edikt governance. Populated by /edikt:adr:compile. -->
[edikt:directives:start]: #
source_hash: 96411f243c413788452d6ce84a0e1316c6f66d2c53fe8f4499ad7b74466cbc57
directives_hash: 73cc6f89a3f2459289bdd715e19a4f1407ccfcb7a8387243a0f5f8dc03b83c6b
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/*.kts"
  - "gradle/libs.versions.toml"
scope:
  - planning
  - design
  - implementation
  - review
directives:
  - "The CQRS / event-sourcing framework MUST be Axon Framework 5.x. NEVER introduce or retain Axon Framework 4.x coordinates in `gradle/libs.versions.toml` or any `build.gradle.kts`. (ref: ADR-001)"
  - "Operations that must atomically enforce an invariant across multiple aggregates MUST use Axon Framework 5's Dynamic Consistency Boundary (DCB) facility. NEVER coordinate such operations with Axon sagas, two-phase commit, distributed transactions, or hand-rolled compensation. (ref: ADR-001)"
  - "All orchestration — long-running multi-step workflows, external-service interactions (third-party HTTP/RPC, OAuth flows, statement imports), durable timers, scheduled reminders, human-in-the-loop steps, retries with backoff, AND in-process reactions to domain events — MUST be implemented as Restate workflows under `adapter/in/restate/**`. (ref: ADR-001)"
  - "Axon sagas MUST NOT be used. NEVER add `@Saga` annotations, and NEVER add classes that implement Axon's `Saga` interface, anywhere under `src/main/**`. (ref: ADR-001)"
  - "Restate workflows MUST interact with the domain only by dispatching Axon commands through Axon's `CommandGateway` and by reading domain state through Axon-backed projections or `QueryGateway`. NEVER write to Axon Server or any event store directly from a Restate handler. (ref: ADR-001)"
  - "Files containing Axon components (`@Aggregate`, `@CommandHandler`, `@EventHandler`, `@EventSourcingHandler`, projection classes) MUST NOT import from `dev.restate.*`. The Axon → Restate dependency direction is forbidden. (ref: ADR-001)"
  - "Imports from `dev.restate.*` MUST be confined to `adapter/in/restate/**`. NEVER import `dev.restate.*` from `domain/**`, `application/**`, `adapter/in/web/**`, or `adapter/out/**`. (ref: ADR-001)"
  - "Restate handlers under `adapter/in/restate/**` MUST contain orchestration only — dispatching Axon commands, invoking external services, scheduling Restate timers — and MUST NOT contain domain logic, aggregate state transitions, or direct read-model writes. (ref: ADR-001)"
reminders:
  - "Before introducing a new cross-aggregate operation → choose DCB (consistency-required) or a Restate workflow (orchestration-required); never reach for a saga — the codebase has no saga lane (ref: ADR-001)"
  - "Before adding a `dev.restate.*` import → confirm the file lives under `adapter/in/restate/**`; Restate code never appears in domain, application, or other adapter directories (ref: ADR-001)"
verification:
  - "[ ] `gradle/libs.versions.toml` pins `axonframework` to a 5.x version; no 4.x Axon coordinates appear in any `build.gradle.kts` (ref: ADR-001)"
  - "[ ] No `@Saga` annotations and no classes implementing Axon's `Saga` interface exist anywhere in `src/main/**` (ref: ADR-001)"
  - "[ ] No file under `domain/**`, `application/**`, `adapter/in/web/**`, or `adapter/out/**` imports from `dev.restate.*` (ref: ADR-001)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
