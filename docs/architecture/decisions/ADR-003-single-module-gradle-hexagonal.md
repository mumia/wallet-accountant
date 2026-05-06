---
status: accepted
date: 2026-05-06
decision-makers: [Miguel Manso]
consulted: []
informed: []
supersedes: null
---

# ADR-003: Single-module Gradle layout for hexagonal architecture

## Context and Problem Statement

The project enforces a hexagonal / DDD layout already documented in `docs/project-context.md` and `docs/guidelines/hexagonal-ddd.md`:

```
src/main/kotlin/<root-package>/
├── domain/         (per-aggregate folders inside)
├── application/    (driving + driven port interfaces, services, projections)
└── adapter/
    ├── in/web/        (REST controllers)
    ├── in/restate/    (Restate workflow handlers)
    └── out/readmodel/ (MongoDB repositories)
```

The hexagonal-ddd guideline already pins layer dependency rules — `domain` depends on nothing; `application` depends only on `domain`; adapters depend on `application` and `domain` and never on each other. INV-002 separately mandates that `domain/` is framework-free.

What's still open is the **build-tooling shape** that backs this layout. Two answers fit the same package conventions:

1. **Single-module Gradle:** one Gradle project, one classpath. The layout is enforced by package conventions plus an architecture test (ArchUnit / Konsist) that fails the build on layer-dependency violations.
2. **Multi-module Gradle:** each layer is a Gradle subproject (`:domain`, `:application`, `:adapter:in:web`, `:adapter:in:restate`, `:adapter:out:readmodel`). The build itself enforces dependency direction — Spring isn't on `:domain`'s classpath, so `domain/` *cannot* import Spring even by accident.

Two adjacent variants also deserve mention since they reframe the layout itself rather than the build:

3. **Clean Architecture 4-layer naming** (`entities` / `use-cases` / `interface-adapters` / `frameworks`) inside a single module.
4. **Screaming / feature-folder architecture** — top-level folders per feature (`account/`, `transaction/`, `category/`), each containing its own hexagonal sub-layers.

How should we organize the build and the source tree to back the hexagonal layout?

## Decision Drivers

- **Operational simplicity for a single-maintainer personal project** — minimize ceremony around builds, IDE setup, and refactors.
- **Continuity with the existing guideline language** — `docs/guidelines/hexagonal-ddd.md` and INV-002 are already written in package-based terms (`**/domain/**`, etc.).
- **Strong static enforcement of layer rules** — whichever build shape we pick, layer violations must fail CI.
- **Reversibility** — keep open a clean path to multi-module if the project graduates to a larger team or codebase.

## Considered Options

1. **Single-module Gradle, package-enforced hexagonal layout** — one Gradle module; layers enforced by package convention plus an ArchUnit / Konsist arch test wired into `./gradlew check`.
2. **Multi-module Gradle, per-layer subprojects** — `:domain`, `:application`, `:adapter:in:web`, `:adapter:in:restate`, `:adapter:out:readmodel`. Layer rules become build-time facts.
3. **Single-module with Clean Architecture 4-layer naming** — same single-module shape, but rename `domain` → `entities`, `application` → `use-cases`, `adapter` → `interface-adapters` / `frameworks`.
4. **Screaming / feature-folder architecture at the top level** — top-level folders per feature, each containing its own layered subfolders.

## Decision Outcome

Chosen option: **Single-module Gradle, package-enforced hexagonal layout**, because it preserves the existing guideline vocabulary, costs the least ceremony for a single-maintainer project, and leans on static analysis (which we need anyway for INV-002) instead of build-time partitioning.

The decision lands as the following hard rules:

- The project MUST be organized as a single Gradle module. The only `include(...)` permitted in `settings.gradle.kts` is for `buildSrc/`. NEVER introduce per-layer or per-adapter Gradle subprojects.
- Hexagonal layers MUST be enforced through package conventions at the `<root-package>` level: `**/domain/**`, `**/application/**`, `**/adapter/in/**`, `**/adapter/out/**`, plus `**/infrastructure/**` for cross-cutting framework wiring (logging, OpenTelemetry, Spring config that doesn't fit an adapter).
- Top-level organization MUST be layer-first. Feature folders SHOULD appear *inside* a layer (e.g., `domain/account/`, `domain/transaction/`, `adapter/out/readmodel/account/`). NEVER create a feature-named directory that contains its own layer subfolders (e.g., `src/main/kotlin/<root>/account/domain/` is forbidden — this is the screaming-architecture pattern, explicitly rejected).
- An ArchUnit or Konsist architecture test MUST exist and enforce, at minimum:
  - `**/domain/**` files import only from `kotlin.*`, `java.time.*`, and other `**/domain/**` types.
  - `**/application/**` files import only from `**/domain/**`, other `**/application/**` types, and a narrow allow-list of frameworks (Axon `org.axonframework.*` interface gateways and `kotlinx.coroutines.*` are typical).
  - `**/adapter/in/**` files do not import other `**/adapter/in/**` peers or any `**/adapter/out/**` package.
  - `**/adapter/out/**` files do not import other `**/adapter/out/**` peers or any `**/adapter/in/**` package.
- The architecture test MUST be wired into `./gradlew check` so CI fails on any layer-dependency violation. NEVER mark the test `@Disabled`, NEVER guard it with `@EnabledIfSystemProperty`, and NEVER allow `--no-tests` to skip it in any pipeline.
- INV-002 (domain framework-free) MUST be enforced by this same architecture test, since single-module Gradle does not provide a build-time classpath fence to fall back on. The arch test is the only enforcement mechanism, and it is non-optional.

### Consequences

**Positive:**
- One build, one test run, one IDE project, one classpath. The whole codebase compiles and tests in a single `./gradlew check` invocation.
- Cross-layer refactors are a single edit pass with no Gradle dependency-graph changes.
- The hexagonal-ddd guideline keeps its current vocabulary and rules verbatim — no rename, no churn.
- Static-analysis enforcement is uniform across all layer rules, INV-002 included; reviewers learn one fence, not two.

**Negative:**
- No build-time isolation. A `domain/` file *can* compile against Spring; the arch test is the only thing preventing it. A misconfigured or accidentally-disabled arch test silently weakens INV-002.
- Per-module incremental compilation isn't available; as the codebase grows, full-classpath rebuilds become slower than per-module ones would be.
- Some layer leaks (e.g., a transitive type in a method signature) are easier to introduce inadvertently than with a hard module boundary.

**Neutral:**
- The arch test becomes load-bearing infrastructure. It is treated as production code: it has tests of its own (positive and negative cases verifying it catches violations), it fails the build on violation, and its disabling requires an ADR amendment.
- The "graduation path" to multi-module remains open: should the project later need build-time isolation (team of 3+, or repeated layer-violation patterns), splitting `domain` into its own subproject is a mechanical refactor — extract the directory, add a `build.gradle.kts` declaring no Spring dependency, replace package references with module dependencies. No code changes required in domain types themselves.

## Pros and Cons of the Options

### 1. Single-module Gradle, package-enforced (chosen)
- ✅ Lowest ceremony — one build file, one IDE project, one classpath.
- ✅ Preserves the existing hexagonal-ddd guideline language without renaming.
- ✅ Cross-layer refactors are fast.
- ❌ No build-time fence; the arch test is the *only* enforcement of INV-002 and layer dependency rules.
- ❌ Build times don't benefit from per-module incremental compilation as the codebase grows.

### 2. Multi-module Gradle, per-layer subprojects
- ✅ Build-time isolation: `domain` module's build script declares no Spring dependency, making INV-002 a physical impossibility to violate.
- ✅ Per-module incremental compilation; faster builds at scale.
- ✅ Module boundaries make the layering visible in IDE project structure and dependency-graph tools.
- ❌ Operational ceremony: every cross-layer refactor touches multiple `build.gradle.kts` files. Adding a port interface requires deciding which module owns it.
- ❌ More moving parts: N module declarations, N test sourcesets, N CI tasks (or a single aggregate task with module-level fan-out).
- ❌ Premature for a single-maintainer project at the current scale (<10 tenants, no team).
- **Rejected because:** the build-time fence is real and valuable, but at the current project scale a passing arch test catches violations equally well. Re-evaluate if the project gains a team or if layer-violation patterns recur in review.

### 3. Single-module with Clean Architecture 4-layer naming
- ✅ Same operational simplicity as option 1.
- ✅ Familiar to developers steeped in Robert Martin's Clean Architecture vocabulary.
- ❌ Renames every package and every guideline phrase from `domain` / `application` / `adapter` to `entities` / `use-cases` / `interface-adapters` / `frameworks`. Pure churn.
- ❌ Doesn't add a single rule the hexagonal model doesn't already give us.
- **Rejected because:** the existing hexagonal vocabulary is the project's pinned language across `project-context.md`, `hexagonal-ddd.md`, INV-002, and ADR-001/ADR-002. Renaming for terminology preference invalidates all of that without changing behavior.

### 4. Screaming / feature-folder architecture at the top level
- ✅ Strong locality per feature — everything an engineer needs to touch for `account/` is in one folder.
- ✅ Trivial feature deletion — drop one directory.
- ❌ Inverts the layering: domain / application / adapter become subfolders of every feature, so cross-cutting concerns (auth filter, JWT-claim filter, shared value objects, the Spring app entry point) lack a clean home.
- ❌ Contradicts `hexagonal-ddd.md`'s "Adapter packages MUST be siblings under `adapter/`" rule directly. Adopting feature folders at the top level requires rewriting that guideline.
- ❌ At the current feature surface (accounts, transactions, categories, budgets), the locality benefit is small and the layer-rule clarity loss is large.
- **Rejected because:** incompatible with the existing layer-first guideline, and the locality benefit doesn't pay for the loss of layer-as-first-class-concept. Feature folders *within* a layer (`domain/account/`, `domain/transaction/`) are still encouraged.

## Confirmation

How we will know this decision is being followed:

- **Build-shape assertion**: `settings.gradle.kts` contains exactly one `rootProject.name = ...` and at most one `include("buildSrc")`. A CI step (`grep -E '^include\(' settings.gradle.kts | grep -v buildSrc`) returns zero matches.
- **Architecture test exists and is wired into `check`**: a Konsist or ArchUnit test class lives in `src/test/kotlin/.../architecture/` and is part of the default `test` task. Removing or disabling it would fail CI on the next push.
- **Architecture-test coverage**: the test asserts (a) `domain` framework isolation per INV-002, (b) layer dependency direction, (c) adapter-to-adapter peer isolation. The test itself has tests / fixtures verifying it would *catch* violations of each rule, not just pass on a clean codebase.
- **Top-level layout assertion**: a forbidden-pattern scan (`find src/main/kotlin/<root-package> -maxdepth 1 -type d`) yields only `domain/`, `application/`, `adapter/`, or `infrastructure/`. Any other top-level directory triggers a CI failure.
- **Manual review**: PRs that introduce a new top-level package under `<root-package>/`, modify `settings.gradle.kts`, or change the architecture-test rules must be reviewed against this ADR and either align with it or amend it.

## More Information

- [INV-002 — Domain has no framework dependencies](../invariants/INV-002-domain-no-framework-dependencies.md)
- [INV-004 — Builds go through the Gradle wrapper](../invariants/INV-004-builds-go-through-wrapper.md)
- [ADR-001 — Axon Framework 5 + Restate division of labor](ADR-001-axon-5-restate-division-of-labor.md)
- `docs/guidelines/hexagonal-ddd.md` — layer dependency rules and package conventions
- Project context: `docs/project-context.md`

<!-- Directives for edikt governance. Populated by /edikt:adr:compile. -->
[edikt:directives:start]: #
source_hash: 87bd327ae82d0e5374325f68248794cff7ef0ef2d63b0fdb13080fcca7f592ff
directives_hash: ae6863728ea7a27ce0071a36d2201554f115641255dd6d3ae35a9a53ca94415e
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/*.kts"
  - "settings.gradle.kts"
  - "**/src/main/kotlin/**"
  - "**/src/test/kotlin/**"
scope:
  - planning
  - design
  - implementation
  - review
directives:
  - "The project MUST be organized as a single Gradle module. The only `include(...)` permitted in `settings.gradle.kts` is for `buildSrc/`. NEVER introduce per-layer or per-adapter Gradle subprojects. (ref: ADR-003)"
  - "Hexagonal layers MUST be enforced through package conventions at the `<root-package>` level: `**/domain/**`, `**/application/**`, `**/adapter/in/**`, `**/adapter/out/**`, plus `**/infrastructure/**` for cross-cutting framework wiring. (ref: ADR-003)"
  - "Top-level organization MUST be layer-first. NEVER create a feature-named directory directly under `<root-package>/` that contains its own layer subfolders (e.g., `<root>/account/domain/`, `<root>/account/adapter/` is forbidden — feature folders MUST live INSIDE a layer, not above it). (ref: ADR-003)"
  - "An ArchUnit or Konsist architecture test MUST exist in `src/test/kotlin/` that enforces: `**/domain/**` files import only from `kotlin.*`, `java.time.*`, and other `**/domain/**` types; `**/application/**` files import only from `**/domain/**`, other `**/application/**` types, and explicitly allow-listed framework packages; `**/adapter/in/**` files do not import other `**/adapter/in/**` peers or any `**/adapter/out/**` package; `**/adapter/out/**` files do not import other `**/adapter/out/**` peers or any `**/adapter/in/**` package. (ref: ADR-003)"
  - "The architecture test MUST be wired into `./gradlew check` so the CI build fails on any layer-dependency violation. NEVER mark the test `@Disabled`, NEVER guard it with `@EnabledIfSystemProperty`, and NEVER allow it to be skipped by `--no-tests` or any other CI flag. (ref: ADR-003)"
  - "INV-002 (domain has no framework dependencies) MUST be enforced by the same architecture test, since single-module Gradle provides no build-time classpath fence to fall back on. The arch test is the only enforcement mechanism for INV-002 and is non-optional. (ref: ADR-003)"
reminders:
  - "Before adding a top-level directory under `src/main/kotlin/<root>/` → confirm it is one of `domain/`, `application/`, `adapter/`, or `infrastructure/`; never invent a feature-named top-level folder that contains its own layer subfolders (ref: ADR-003)"
  - "Before editing `settings.gradle.kts` → verify you are not adding `include(...)` for new subprojects; the project is single-module by design, the only allowed include is `buildSrc/` (ref: ADR-003)"
verification:
  - "[ ] `settings.gradle.kts` contains no `include(...)` lines except for `buildSrc/` (`grep -E '^include\\(' settings.gradle.kts | grep -v buildSrc` returns no matches) (ref: ADR-003)"
  - "[ ] An ArchUnit / Konsist architecture test exists in `src/test/kotlin/` and is wired into `./gradlew check`; the test is not `@Disabled` and is not gated by any system-property or environment guard (ref: ADR-003)"
  - "[ ] The only top-level directories under `src/main/kotlin/<root-package>/` are `domain/`, `application/`, `adapter/`, and (optionally) `infrastructure/`; no feature-named top-level directory contains its own layer subfolders (ref: ADR-003)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
