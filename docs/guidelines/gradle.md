# Gradle Guidelines

**Purpose:** Keep the Gradle build reproducible, fast, and consistent across modules — preventing version drift, leaky module boundaries, configuration-cache regressions, and the per-script divergence that turns multi-module Gradle builds into a maintenance burden.

## Rationale

Gradle's flexibility is its largest footgun. Without explicit conventions, each `build.gradle.kts` diverges, dependency versions drift between modules, builds stop being reproducible across developer machines and CI, and incremental performance silently rots as build logic violates the configuration cache contract. The rules below pin the conventions that the Gradle team itself recommends: Kotlin DSL for type safety, version catalogs as the single source of truth for coordinates, lock files for transitive reproducibility, the `plugins { }` block for declarative plugin resolution, the `java-library` `api`/`implementation` separation for honest module surfaces, and convention plugins (instead of root-level `subprojects { }` mutation) so build logic stays inspectable per-module.

In this project (Kotlin + Spring Boot + Axon Framework 5 + Restate, multi-module hexagonal), the build coordinates several plugin ecosystems (Spring Boot, Kotlin JVM, Kotlin Spring, Axon) across at least three layers (domain, application, adapters). The rules are calibrated to keep those modules independently buildable, the dependency surface honest, and the version of each framework component pinned deterministically — so that "what runs in CI" and "what the developer just built" cannot diverge.

## Rules

- MUST write every Gradle build script in Kotlin DSL (`build.gradle.kts`, `settings.gradle.kts`). NEVER introduce Groovy DSL (`build.gradle`, `settings.gradle`) or mix the two within the project.
- MUST declare every external dependency and plugin version in a single Gradle version catalog at `gradle/libs.versions.toml`. NEVER hardcode dependency coordinates, version literals, or plugin versions inside any `build.gradle.kts` file.
- MUST apply plugins via the `plugins { }` block, sourcing each plugin from the version catalog (`alias(libs.plugins.X)`) when one is defined. NEVER use the legacy `apply plugin: "..."` / `buildscript { classpath(...) }` syntax in first-party build scripts.
- MUST use `implementation` for dependencies that are NOT exposed on a module's public API and reserve `api` exclusively for types that appear in a public class, function, or property signature of that module. NEVER use the deprecated `compile` configuration, and NEVER default to `api` to "make things work".
- MUST commit `gradlew`, `gradlew.bat`, and `gradle/wrapper/gradle-wrapper.properties` with a pinned `distributionUrl` and `distributionSha256Sum`. NEVER invoke a system-installed `gradle` binary — every build, CI step, and developer command MUST go through `./gradlew`.
- MUST declare all repositories centrally in `settings.gradle.kts` via `dependencyResolutionManagement { repositories { ... } }` with `repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)`. NEVER declare `repositories { }` in subproject `build.gradle.kts` files.
- MUST commit Gradle dependency locks (`gradle.lockfile` per project, or per-configuration locks under `gradle/dependency-locks/`) for every configuration that contributes to the runtime classpath. NEVER use unbounded version ranges (`+`, `latest.release`, `1.2.+`, dynamic versions) anywhere in the catalog or build scripts.
- MUST keep all custom build logic compatible with Gradle's configuration cache: no `Project` access at execution time, no `Task` references captured in execution-time lambdas, no static mutable state across tasks. NEVER call `project.exec` / `project.javaexec` from a task action — inject `ExecOperations` (or `FileSystemOperations`, `ArchiveOperations`) via `@Inject` instead.
- MUST encapsulate shared build logic in convention plugins under `buildSrc/` or an included `build-logic` build, applied to each module via the `plugins { }` block. NEVER configure other modules from the root `build.gradle.kts` using `subprojects { }`, `allprojects { }`, or cross-project property mutation.
- MUST resolve transitive version conflicts deliberately — via version catalog bundles, `dependencies { constraints { ... } }`, or `configurations.all { resolutionStrategy.failOnVersionConflict() }` — and record every override with an inline comment naming the reason. NEVER rely on Gradle's silent "highest version wins" default for runtime-classpath dependencies.

## When NOT to apply

These rules do not apply to:

- **Throwaway / one-off scripts** that are not committed to the repository. An `init.gradle.kts` used locally for a single experimental build, or a personal `~/.gradle/init.d/` script, may diverge from these rules — but the moment a script is committed, every rule applies.
- **Buildless prototypes** before any module split exists. Rule 9 (convention plugins under `buildSrc/`) MAY be deferred while the project is a single-module prototype with no shared build logic to extract; it MUST be applied as soon as a second module is added.
- **Third-party plugins absent from the version catalog**. A plugin outside the Gradle Plugin Portal that has no catalog entry yet MAY be applied via `id("...") version "..."` directly in the `plugins { }` block as a temporary measure, with a tracking TODO. It MUST NEVER be applied via legacy `apply plugin:` syntax, and the catalog entry MUST be added in the same change set or in a follow-up tracked in the issue tracker.

These three exceptions are the only legitimate ones. "It's faster to inline the version", "the plugin block is annoying here", "we'll catalogize later" are not exceptions — every dependency and plugin version MUST live in the catalog from day one.

---

*Created by edikt:guideline — 2026-04-26*

<!-- Directives for edikt governance. Populated by /edikt:guideline:compile. -->
[edikt:directives:start]: #
source_hash: 647dd07ac21b077769f66ee2aebb5ca59aab04bddcbf835d302fc7e4168b1844
directives_hash: 1f3a2f48768c930ad4082559497d9148354960d4b632960ce2c67a6cd35d25ef
compiler_version: "0.4.3"
paths:
  - "**/*.gradle.kts"
  - "**/*.gradle"
  - "gradle/**"
  - "gradlew"
  - "gradlew.bat"
scope:
  - implementation
  - review
directives:
  - "MUST write every Gradle build script in Kotlin DSL (`build.gradle.kts`, `settings.gradle.kts`). NEVER introduce Groovy DSL (`build.gradle`, `settings.gradle`) or mix the two within the project. (ref: gradle)"
  - "MUST declare every external dependency and plugin version in a single Gradle version catalog at `gradle/libs.versions.toml`. NEVER hardcode dependency coordinates, version literals, or plugin versions inside any `build.gradle.kts` file. (ref: gradle)"
  - "MUST apply plugins via the `plugins { }` block, sourcing each plugin from the version catalog (`alias(libs.plugins.X)`) when one is defined. NEVER use the legacy `apply plugin: \"...\"` / `buildscript { classpath(...) }` syntax in first-party build scripts. (ref: gradle)"
  - "MUST use `implementation` for dependencies that are NOT exposed on a module's public API and reserve `api` exclusively for types that appear in a public class, function, or property signature of that module. NEVER use the deprecated `compile` configuration, and NEVER default to `api` to \"make things work\". (ref: gradle)"
  - "MUST commit `gradlew`, `gradlew.bat`, and `gradle/wrapper/gradle-wrapper.properties` with a pinned `distributionUrl` and `distributionSha256Sum`. NEVER invoke a system-installed `gradle` binary — every build, CI step, and developer command MUST go through `./gradlew`. (ref: gradle)"
  - "MUST declare all repositories centrally in `settings.gradle.kts` via `dependencyResolutionManagement { repositories { ... } }` with `repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)`. NEVER declare `repositories { }` in subproject `build.gradle.kts` files. (ref: gradle)"
  - "MUST commit Gradle dependency locks (`gradle.lockfile` per project, or per-configuration locks under `gradle/dependency-locks/`) for every configuration that contributes to the runtime classpath. NEVER use unbounded version ranges (`+`, `latest.release`, `1.2.+`, dynamic versions) anywhere in the catalog or build scripts. (ref: gradle)"
  - "MUST keep all custom build logic compatible with Gradle's configuration cache: no `Project` access at execution time, no `Task` references captured in execution-time lambdas, no static mutable state across tasks. NEVER call `project.exec` / `project.javaexec` from a task action — inject `ExecOperations` (or `FileSystemOperations`, `ArchiveOperations`) via `@Inject` instead. (ref: gradle)"
  - "MUST encapsulate shared build logic in convention plugins under `buildSrc/` or an included `build-logic` build, applied to each module via the `plugins { }` block. NEVER configure other modules from the root `build.gradle.kts` using `subprojects { }`, `allprojects { }`, or cross-project property mutation. (ref: gradle)"
  - "MUST resolve transitive version conflicts deliberately — via version catalog bundles, `dependencies { constraints { ... } }`, or `configurations.all { resolutionStrategy.failOnVersionConflict() }` — and record every override with an inline comment naming the reason. NEVER rely on Gradle's silent \"highest version wins\" default for runtime-classpath dependencies. (ref: gradle)"
reminders:
  - "Before adding a dependency → declare its coordinates and version in `gradle/libs.versions.toml`, never inline in a build script (ref: gradle)"
  - "Before declaring a dependency in a module → use `implementation` unless the type is part of the module's public API; reserve `api` for re-exported types (ref: gradle)"
verification:
  - "[ ] No hardcoded versions in `build.gradle.kts` — every coordinate is referenced via `libs.` from the catalog (ref: gradle)"
  - "[ ] No legacy `apply plugin:` or `buildscript { classpath(...) }` syntax in any `*.gradle.kts` file (ref: gradle)"
  - "[ ] No dynamic versions (`+`, `latest.release`, `1.2.+`) in `gradle/libs.versions.toml` or build scripts (ref: gradle)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
