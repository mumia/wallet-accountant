# wallet-accountant — Project Context

## What This Is

A multi-tenant application for managing personal accounts — "Your Accountant in your Wallet". Each tenant operates an isolated view of their accounts, transactions, and categorisation. The system is event-sourced and uses CQRS, with durable execution for any long-running or cross-aggregate workflow.

## Stack

- **Language / build:** Kotlin (Gradle)
- **Application framework:** Spring (Spring Boot)
- **CQRS / Event Sourcing:** Axon Framework 5
- **Event store:** Axon Server
- **Read models:** MongoDB
- **Durable Execution Engine:** [Restate](https://www.restate.dev) — for any workflow that must survive process restarts, retries, or partial failures (sagas, multi-step orchestration, externally observable side effects).

## Architecture

Hexagonal / DDD with three layers and a strict folder layout:

### Domain Layer
- One folder per aggregate, each containing: `entities/`, `value-objects/`, `commands/`, `queries/`, `events/`.
- A shared `value-objects/` folder for VOs used across multiple aggregates.

### Application Layer
- Port interfaces — split into **driving** (`in`) and **driven** (`out`) ports.
- Command interceptors.
- Read models.
- Projections.
- Query handlers.
- Services.

### Adapter Layer
- Concrete implementations of the port interfaces, organised by direction:
  - `in/web/` — REST API adapters (driving).
  - `in/restate/` — Restate service handlers (driving).
  - `out/readmodel/` — MongoDB read repositories (driven).
- Future driving adapters (e.g. message consumers) live alongside `in/web` and `in/restate`. Future driven adapters (e.g. external HTTP clients) live alongside `out/readmodel`.

## Users

Multi-tenant — each tenant is an end user managing their own personal accounts. Tenant isolation is a hard architectural constraint and applies to:
- Every query against the read models in MongoDB (must be tenant-scoped).
- Every command against an aggregate (must verify tenant ownership before applying state changes).
- Every Restate workflow (the workflow context must carry the tenant identity end-to-end).

## Notes

- **No Kotlin / Spring / Axon rule packs ship with edikt.** Stack-specific conventions are captured as **guidelines** in `docs/guidelines/` via `/edikt:guideline:new` and compiled into governance directives.
- **Architecture enforcement:** consider adding [verikt](https://verikt.dev) once the project has shape — edikt rules catch issues at prompt time, verikt catches them in CI.

---

*Initialized by edikt: 2026-04-25*
