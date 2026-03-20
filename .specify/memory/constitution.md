# Project Constitution — Wallet Accountant

> Version: v1.5.1
> Ratified: 2026-03-06
> Last Amended: 2026-03-20

## Principles

### 1. Event Sourcing First

All state changes MUST be modeled as domain events. Aggregates MUST NOT expose mutable state; Axon Server is the single source of truth for event history.

### 2. CQRS Separation

Command and query sides MUST be strictly separated. Command handlers produce events; projections consume events to build MongoDB read models. No direct writes to read models from command handlers.

### 3. Aggregate Design

Each aggregate MUST protect a single consistency boundary. Commands MUST be validated within the aggregate before emitting events. Aggregate classes MUST remain small and focused.

### 4. Test-Driven Development

Tests MUST be written before implementation (Red-Green-Refactor). Axon's test fixtures MUST be used for aggregate testing. Projection tests MUST verify event-to-read-model mapping.

### 5. Simplicity

Start with the simplest design that satisfies requirements (YAGNI). Avoid premature abstractions, unnecessary layers, or speculative features.

### 6. Hexagonal Architecture

The application MUST follow hexagonal architecture with strict separation of driving (in) and driven (out) ports and adapters. All external interactions MUST flow through port interfaces.

### 7. Inversion of Control

Dependencies MUST be injected, never instantiated directly. All components MUST depend on abstractions (port interfaces), not concrete implementations.

### 8. Dynamic Consistency Boundary

Operations spanning multiple aggregates MUST use Axon Framework 5's Dynamic Consistency Boundary concept. Direct cross-aggregate command dispatching from within an aggregate is forbidden.

### 9. Immutability

All objects (entities, value objects, commands, events, queries, read models) MUST be immutable. State changes MUST produce new instances; no in-place mutation.

### 10. No Layer Proliferation

New architectural layers MUST NOT be introduced unless explicitly required by a feature specification. The project's layers (Domain, Application, Adapter) as defined in the Project Structure section are the only permitted layers. If a new layer appears justified, a clarification question MUST be raised before proceeding to confirm the need, scope, and placement of the proposed layer.

### 11. Backend Only

This application is strictly backend. No frontend code, UI frameworks, or client-side rendering MUST be introduced. All driving adapters expose programmatic APIs (e.g., REST), not user-facing interfaces.

### 12. JSON Serializability

All objects persisted as part of an event or command MUST be serializable and deserializable to/from JSON. This includes the events and commands themselves, as well as any nested value objects or entities they contain. Types that cannot round-trip through JSON serialization MUST NOT appear in command or event payloads.

### 13. RFC 7807 Error Responses

All API error responses MUST conform to the RFC 7807 (Problem Details for HTTP APIs) format. Every error returned by a REST endpoint MUST include at minimum the `type`, `title`, and `status` fields. Custom extensions are permitted but the base structure MUST NOT deviate from the RFC 7807 specification.

<!--
Sync Impact Report
- Version change: v1.5.0 → v1.5.1
- Added principles: none
- Modified principles: #10 No Layer Proliferation (added clarification question requirement)
- Added sections: none
- Removed sections: none
- Templates requiring updates:
  - `.specify/templates/plan-template.md` ✅ no update needed (generic)
  - `.specify/templates/spec-template.md` ✅ no update needed (generic)
  - `.specify/templates/tasks-template.md` ✅ no update needed (generic)
- Follow-up TODOs: none
-->

## Project Structure

### Domain Layer

- One folder per aggregate containing: entities, value objects, commands, queries, events
- A shared value objects folder for value objects used across multiple aggregates

### Application Layer

- Port interfaces: driving (in) ports and driven (out) ports
- Command interceptors
- Read models
- Projections
- Query handlers
- Services

### Adapter Layer

- Concrete implementations of port interfaces for both in and out directions
- `in/web` — REST API adapters (driving)
- `out/readmodel` — Database read repositories (driven)

## Technology Constraints

- **Language:** Kotlin
- **Build system:** Gradle
- **Framework:** Axon Framework
- **Event store:** Axon Server
- **Read model DB:** MongoDB

No alternative persistence or messaging without constitutional amendment.

## Development Workflow

- Feature branches for all work
- Spec-driven development via speckit workflow
- Commit after each logical unit of work

## Governance

- This constitution supersedes conflicting practices
- Amendments require a version bump and documentation of the change
- CLAUDE.md is used for runtime development guidance and tooling configuration
