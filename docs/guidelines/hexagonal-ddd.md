# Hexagonal / DDD Guidelines

**Purpose:** Keep the architecture honest — preserve a framework-agnostic domain, route every external interaction through explicit ports, place behavior on the domain objects that own the data, and prevent the slow drift toward anemic models, leaky adapters, and god services that erodes hexagonal/DDD codebases over time.

## Rationale

Hexagonal architecture (Cockburn, 2005) and tactical DDD (Evans 2003, Vernon 2013) share a single goal: protect the domain from everything else. Frameworks come and go (Spring versions, Mongo drivers, HTTP libraries); the domain language and the invariants the business cares about should outlive all of them. This guideline pins the structural rules that make that possible — strict layer separation (domain → application → adapter), ports defined at the application boundary, adapters as peers under `adapter/in/*` and `adapter/out/*`, and behavior placed on the entities, value objects, and aggregate roots that own the relevant state.

The DDD building blocks reinforce the same boundary from the inside: aggregates with a single root keep state transitions concentrated and invariants enforceable; value objects modeled as immutable `data class`es with constructor-time validation make "invalid state" unrepresentable; CQS keeps read paths from contaminating write paths and makes the read-model adapter (Mongo projections) a clean projection rather than a leaky cache. The rules below are the project's contract for staying in this regime as the codebase grows.

## Rules

- The application MUST follow hexagonal architecture with strict separation of driving (in) and driven (out) ports and adapters.
- All external interactions MUST flow through port interfaces.
- The domain layer MUST be free of framework, library, persistence, transport, and serialization concerns. NEVER import Spring, Spring Data Mongo, Restate, HTTP, Jackson, or messaging-framework types into a class under the `domain/` package.
- Driving (in) ports MUST be defined as interfaces in the `application/` layer; driving adapters (`adapter/in/web`, `adapter/in/restate`, `adapter/in/cli`, etc.) MUST depend on those interfaces to enter the application. NEVER let a driving adapter call another adapter directly or skip the application layer.
- Driven (out) ports MUST be defined as interfaces in the `application/` layer; driven adapters (`adapter/out/readmodel`, `adapter/out/http`, etc.) MUST implement those interfaces to leave the application. NEVER let application code reference a driven adapter implementation directly — depend on the port interface.
- Adapter packages MUST be siblings under `adapter/` and NEVER reference each other directly. Cross-adapter coordination MUST go through the application layer.
- Domain behavior MUST live on the entity, value object, or aggregate root that owns the relevant data. NEVER write `*Service` classes whose methods only orchestrate getters and setters on a domain object — that is the anemic domain model anti-pattern.
- Aggregates MUST have a single root entity that is the only entry point for changing the aggregate's state. NEVER mutate an aggregate's internal state through any path other than the aggregate root's public API.
- Aggregate invariants MUST be enforced inside the aggregate root itself. Every state transition that must satisfy a domain rule MUST be implemented in the aggregate's behavior, not in a service, handler, or controller.
- Value objects MUST be immutable Kotlin `data class`es with `val` properties and MUST validate their invariants in the constructor (`init { require(...) }`) or a factory function. NEVER let a value object exist in an invalid state, and NEVER add setters or `var` properties to a value object.
- Application service methods MUST separate commands from queries (CQS): methods that mutate state accept Command objects and return events; methods that read state accept Query objects and return read-model DTOs. NEVER mix command and query responsibilities in a single application service method.

## When NOT to apply

These rules do not apply to:

- **Generated code** — Axon-generated classes, Spring-generated proxies, KSP/KAPT output, and any code emitted by an annotation processor or build plugin. Generators emit code that does not respect hexagonal layering by design.
- **Migration shims at the edge of integrations** — adapters that bridge to a third-party SDK whose types must leak slightly beyond the adapter package (e.g., a callback type the SDK requires) MAY hold the SDK type for one layer of indirection, provided the shim is explicitly named (`*Bridge`, `*Adapter`) and a port-shaped translation exists immediately downstream.
- **Initial spike and proof-of-concept code** under a clearly marked package or branch (e.g., `spike/`, `prototype/`). Spikes MAY collapse layers temporarily, but production code paths MUST be re-extracted into the proper hexagonal structure before merging to `main`.
- **Cross-cutting infrastructure** that is genuinely framework-bound (logging facade configuration, transaction-manager wiring, OpenTelemetry instrumentation). These live in `adapter/` or `infrastructure/` packages — they do not violate the rule because they were never claiming to be domain code.

These four exceptions are the only legitimate ones. "It's faster to put it in the service for now", "the domain just needs one Spring annotation", and "the adapter only references the other adapter for one method" are not exceptions — they are exactly the failure modes this guideline exists to prevent.

---

*Created by edikt:guideline — 2026-04-30*

<!-- Directives for edikt governance. Populated by /edikt:guideline:compile. -->
[edikt:directives:start]: #
source_hash: 5a9a23c1d786ac2f27e62a15d12270bf5aa0f95ec474cf0cbbfc0fb5068cff0c
directives_hash: 4bcd10ae643ebf13ab91f2f06f95ddd78bfd0ca7140496a4c4868e5d7d32f72d
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/domain/**"
  - "**/application/**"
  - "**/adapter/**"
scope:
  - design
  - implementation
  - review
directives:
  - "The application MUST follow hexagonal architecture with strict separation of driving (in) and driven (out) ports and adapters. (ref: hexagonal-ddd)"
  - "All external interactions MUST flow through port interfaces. (ref: hexagonal-ddd)"
  - "The domain layer MUST be free of framework, library, persistence, transport, and serialization concerns. NEVER import Spring, Spring Data Mongo, Restate, HTTP, Jackson, or messaging-framework types into a class under the `domain/` package. (ref: hexagonal-ddd)"
  - "Driving (in) ports MUST be defined as interfaces in the `application/` layer; driving adapters (`adapter/in/web`, `adapter/in/restate`, `adapter/in/cli`, etc.) MUST depend on those interfaces to enter the application. NEVER let a driving adapter call another adapter directly or skip the application layer. (ref: hexagonal-ddd)"
  - "Driven (out) ports MUST be defined as interfaces in the `application/` layer; driven adapters (`adapter/out/readmodel`, `adapter/out/http`, etc.) MUST implement those interfaces to leave the application. NEVER let application code reference a driven adapter implementation directly — depend on the port interface. (ref: hexagonal-ddd)"
  - "Adapter packages MUST be siblings under `adapter/` and NEVER reference each other directly. Cross-adapter coordination MUST go through the application layer. (ref: hexagonal-ddd)"
  - "Domain behavior MUST live on the entity, value object, or aggregate root that owns the relevant data. NEVER write `*Service` classes whose methods only orchestrate getters and setters on a domain object — that is the anemic domain model anti-pattern. (ref: hexagonal-ddd)"
  - "Aggregates MUST have a single root entity that is the only entry point for changing the aggregate's state. NEVER mutate an aggregate's internal state through any path other than the aggregate root's public API. (ref: hexagonal-ddd)"
  - "Aggregate invariants MUST be enforced inside the aggregate root itself. Every state transition that must satisfy a domain rule MUST be implemented in the aggregate's behavior, not in a service, handler, or controller. (ref: hexagonal-ddd)"
  - "Value objects MUST be immutable Kotlin `data class`es with `val` properties and MUST validate their invariants in the constructor (`init { require(...) }`) or a factory function. NEVER let a value object exist in an invalid state, and NEVER add setters or `var` properties to a value object. (ref: hexagonal-ddd)"
  - "Application service methods MUST separate commands from queries (CQS): methods that mutate state accept Command objects and return events; methods that read state accept Query objects and return read-model DTOs. NEVER mix command and query responsibilities in a single application service method. (ref: hexagonal-ddd)"
reminders:
  - "Before adding a class to the domain layer → confirm zero framework/persistence/transport imports (no Spring, Spring Data Mongo, Restate, HTTP, Jackson) (ref: hexagonal-ddd)"
  - "Before adding business behavior → place it on the entity, value object, or aggregate root that owns the data — never inside a `*Service` (ref: hexagonal-ddd)"
verification:
  - "[ ] No framework imports (Spring, Spring Data Mongo, Restate, HTTP, Jackson, messaging frameworks) in any file under `**/domain/**` (ref: hexagonal-ddd)"
  - "[ ] No adapter-to-adapter imports — files under `adapter/in/**` and `adapter/out/**` do not reference one another (ref: hexagonal-ddd)"
  - "[ ] No `var` properties, setters, or mutable collections on value objects under `**/domain/**` (ref: hexagonal-ddd)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
