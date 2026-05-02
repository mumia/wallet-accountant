# INV-004: Builds always go through the project Gradle wrapper

**Date:** 2026-05-01
**Status:** Active

## Statement

Every Gradle invocation — local development, CI, IDE-driven, Docker build, release pipeline — runs through the project's pinned Gradle wrapper (`./gradlew`), with `gradle/wrapper/gradle-wrapper.properties` declaring both a specific `distributionUrl` version and a non-empty `distributionSha256Sum`.

## Rationale

A system-installed `gradle` is whatever version happens to live on a developer's machine or a CI runner image. Different versions have different bytecode targets, different deprecation timing, different Kotlin DSL behaviors, and different bug profiles. Reproducibility — the entire reason for version-locking dependencies and plugins — depends on every invocation using the same Gradle distribution, verified by SHA-256. Without the wrapper, "works on my machine" becomes a routine excuse and the project loses any meaningful claim to reproducible builds.

## Consequences of violation

- Build divergence: passes locally on Gradle 8.x, fails on CI's Gradle 9.x (or vice versa), and the cause is invisible because both invocations look identical in the logs.
- Reproducibility claim becomes false; supply-chain attestations and audit findings against `SLSA` / `in-toto` / similar frameworks fail because the build environment can't be pinned.
- Deprecation drift: warnings on one Gradle version become errors on another, breaking PRs intermittently as contributors update their local Gradle independently.
- Onboarding friction: new contributors must hunt the right Gradle version manually — and frequently install the wrong one, producing silent bugs in their first PR.
- Build cache poisoning: differently-versioned Gradle invocations populate the local and remote build caches with incompatible artifacts, causing intermittent failures that look like flaky tests.

## Implementation

The repository tracks `gradlew`, `gradlew.bat`, `gradle/wrapper/gradle-wrapper.jar`, and `gradle/wrapper/gradle-wrapper.properties` with a pinned `distributionUrl` (specific version, no `-latest` / `-rc`) and a `distributionSha256Sum` for integrity verification. CI pipelines, `Dockerfile`s, IDE run configurations, README instructions, and any shell scripts that invoke the build all reference `./gradlew`, never bare `gradle`. The wrapper itself is upgraded explicitly via `./gradlew wrapper --gradle-version=X.Y --distribution-type=bin` and the resulting properties / jar changes go through normal review. The gradle guideline captures the operational mechanism.

## Anti-patterns

- A `Dockerfile` line `RUN apt-get install gradle && gradle build` — uses the system gradle, ignores the wrapper, defeats the entire point of pinning.
- A `Makefile` target `build:` followed by `gradle assemble` — same problem with a friendlier veneer.
- CI YAML invoking `gradle test` instead of `./gradlew test` — most common single source of "passed locally, failed in CI".
- Stripping `gradlew*` and `gradle/wrapper/` from a checkout to "save space" before running the build, or excluding them from a Docker context to "speed up COPY".
- A developer alias `alias gradle=/usr/local/bin/gradle` in their shell so `./gradlew` is silently overridden by the system binary.
- A `gradle-wrapper.properties` with `distributionUrl=...gradle-bin.zip` (no version pin) or with `distributionSha256Sum` removed "because Gradle complained once" — both defeat the integrity guarantee.

## Enforcement

- **Automated (CI invocation check)**: every CI workflow file (`.github/workflows/*.yml`, `Jenkinsfile`, etc.) invokes `./gradlew`; a CI lint step or repository structural check fails the pipeline if a bare `gradle` invocation appears in any workflow file.
- **Automated (wrapper integrity)**: a pre-commit hook and a CI structural check verify that `gradlew`, `gradlew.bat`, `gradle/wrapper/gradle-wrapper.jar`, and `gradle/wrapper/gradle-wrapper.properties` are tracked, that the properties file contains a `distributionUrl` pointing to a specific Gradle version (regex match on `gradle-\d+\.\d+(\.\d+)?-bin\.zip`), and that `distributionSha256Sum` is present and non-empty.
- **Automated (forbidden-pattern scan)**: a CI step greps `**/Dockerfile`, `**/*.sh`, `**/*.yml` (CI), `**/Makefile`, `**/README*.md`, and `**/CONTRIBUTING*.md` for any standalone `gradle ` invocation followed by a build verb (`build`, `test`, `assemble`, `check`, `clean`) and flags it for review.
- **Manual**: PR reviewers reject any change that removes the wrapper, weakens the version pin, removes the SHA-256 sum, or scripts the build with bare `gradle`.

<!-- Directives for edikt governance. Populated by /edikt:invariant:compile. -->
[edikt:directives:start]: #
source_hash: bfe8bd39b866c72162846e8cfadeee1d55b387c2d7de9c123df72fe3cebe9b46
directives_hash: 802f560b2c4825aeae11711c52c3aa7f504882161e2257fe1485e1f16e3e29b9
compiler_version: "0.4.3"
paths:
  - "**/*.gradle.kts"
  - "**/*.gradle"
  - "**/gradle/**"
  - "**/gradlew"
  - "**/gradlew.bat"
  - "**/.github/workflows/**"
  - "**/Jenkinsfile"
  - "**/Dockerfile"
  - "**/*.sh"
  - "**/Makefile"
  - "**/README*.md"
  - "**/CONTRIBUTING*.md"
scope:
  - design
  - implementation
  - review
directives:
  - "Every Gradle invocation — local, CI, Dockerfile, IDE configuration, release pipeline — MUST go through the project's pinned wrapper at `./gradlew` (with `gradle/wrapper/gradle-wrapper.properties` containing both a `distributionUrl` to a specific Gradle version and a non-empty `distributionSha256Sum`). NEVER invoke a system-installed `gradle` binary, NEVER ship a checkout missing `gradlew`, `gradlew.bat`, or the wrapper properties, and NEVER weaken the version pin to `-latest` or remove the SHA-256 sum. (ref: INV-004)"
reminders:
  - "Before invoking Gradle from a script, Dockerfile, or CI workflow → use `./gradlew`, never bare `gradle` (ref: INV-004)"
verification:
  - "[ ] All CI workflows, `Dockerfile`s, `Makefile`s, and shell scripts invoke `./gradlew`; no bare `gradle <verb>` invocations anywhere in the repository (ref: INV-004)"
  - "[ ] `gradle/wrapper/gradle-wrapper.properties` has `distributionUrl` pinned to a specific Gradle version (no `-latest`, no `-rc`) and a non-empty `distributionSha256Sum` (ref: INV-004)"
  - "[ ] `gradlew`, `gradlew.bat`, `gradle/wrapper/gradle-wrapper.jar`, and `gradle/wrapper/gradle-wrapper.properties` are all tracked in version control (ref: INV-004)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
