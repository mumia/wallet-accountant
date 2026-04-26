# Kotlin Guidelines

**Purpose:** Keep Kotlin code idiomatic and safe — null safety, immutability, sealed types, and structured concurrency are the language's load-bearing features, and bypassing them re-introduces the Java pitfalls Kotlin was designed to eliminate.

## Rationale

Kotlin's value over Java is concentrated in a small set of features: nullability tracked in the type system, `val`/read-only collections by default, exhaustive `when` over sealed hierarchies, `data class` for value semantics, and structured coroutines. Code that ignores these features compiles fine but loses every guarantee that justified adopting Kotlin in the first place — `!!` re-introduces NPEs, `MutableList` on public APIs leaks shared mutable state, broad `catch (e: Exception)` swallows real bugs, and `GlobalScope` produces leaked work that survives the request that started it. These rules exist to make the safe path the default path; deviations should be conscious and local, not accidental and pervasive.

In this project (Spring + Axon Framework 5 + Restate, hexagonal/DDD), these defaults compound: aggregates, commands, and events benefit from `data class` semantics; read-side projections benefit from immutable collections; durable execution and event handlers benefit from explicit dispatcher choice and structured scopes. The rules are written to apply uniformly across the stack.

## Rules

- MUST use `val` for declarations unless control flow requires reassignment. NEVER declare a `var` that is only assigned once.
- NEVER use the non-null assertion operator (`!!`) in production code. Use safe calls (`?.`), `requireNotNull(x) { "..." }`, `checkNotNull(x) { "..." }`, or an explicit null branch with a meaningful error message.
- MUST model absence with nullable types (`T?`). NEVER use sentinel values (empty string, `-1`, `"N/A"`) to represent "no value".
- MUST declare data carriers — DTOs, value objects, Axon commands, and Axon events — as `data class`. NEVER hand-roll `equals`, `hashCode`, `toString`, or `copy` when `data class` would generate them correctly.
- MUST expose read-only collection types (`List`, `Set`, `Map`) across module, package, or layer boundaries. NEVER expose `MutableList`, `MutableSet`, or `MutableMap` from a public API.
- NEVER catch generic `Exception`, `Throwable`, or `RuntimeException` in business logic. Catch the specific subtype, or use `runCatching { ... }` and handle the resulting `Result` with `onFailure` / `getOrElse`.
- MUST model branching domain outcomes with `sealed interface` or `sealed class` and exhaustive `when` (no `else` branch on a sealed hierarchy). NEVER use exceptions as control flow for expected business outcomes.
- MUST launch coroutines from a structured `CoroutineScope` tied to an explicit lifecycle. NEVER call `GlobalScope.launch` or `GlobalScope.async` in application code.
- MUST switch to `Dispatchers.IO` for blocking I/O inside suspending functions via `withContext(Dispatchers.IO) { ... }`. NEVER perform blocking JDBC, file, or network calls on `Dispatchers.Default` or `Dispatchers.Main`.
- MUST use Spring constructor injection (`class Foo(private val bar: Bar)`). NEVER use field injection (`@Autowired lateinit var ...` or `@Autowired` on mutable properties).

## When NOT to apply

These rules do not apply to:

- **Generated code** under `build/`, `generated/`, or any path written by a code generator (Axon's generated classes, Kotlin Symbol Processing output, OpenAPI-generated clients). Generators emit non-idiomatic code by design and rewriting it defeats regeneration.
- **Test code** under `src/test/` and `src/integrationTest/`. The `!!` operator and broader exception catches MAY be used in tests where the failing path is the assertion itself — `actual!!.field` inside an assertion is acceptable because a failed test is the intended failure mode.
- **Java interop boundaries**. When bridging to a Java API that returns platform types (`String!`) or throws checked exceptions, the rules relax for the single layer that owns the bridge — but that layer MUST translate to nullable Kotlin types and Kotlin-idiomatic results before exposing them further.

These three exceptions are the only legitimate ones. "Convenience", "time pressure", "the rule is annoying here" are not exceptions — if a rule does not fit a class of code that is not generated, test, or interop, raise it for revision rather than silently violating it.

---

*Created by edikt:guideline — 2026-04-26*

<!-- Directives for edikt governance. Populated by /edikt:guideline:compile. -->
[edikt:directives:start]: #
source_hash: 4a1ccabe151356941d6e9163bcf5908a4771cf600e0a254b4782db071e83040a
directives_hash: 0077bc9ac445cc874450daf1fc8cd1aec3401d82936432233de2d28d22b67e96
compiler_version: "0.4.3"
paths:
  - "**/*.kt"
  - "**/*.kts"
scope:
  - implementation
  - review
directives:
  - "MUST use `val` for declarations unless control flow requires reassignment. NEVER declare a `var` that is only assigned once. (ref: kotlin)"
  - "NEVER use the non-null assertion operator (`!!`) in production code. Use safe calls (`?.`), `requireNotNull(x) { \"...\" }`, `checkNotNull(x) { \"...\" }`, or an explicit null branch with a meaningful error message. (ref: kotlin)"
  - "MUST model absence with nullable types (`T?`). NEVER use sentinel values (empty string, `-1`, `\"N/A\"`) to represent \"no value\". (ref: kotlin)"
  - "MUST declare data carriers — DTOs, value objects, Axon commands, and Axon events — as `data class`. NEVER hand-roll `equals`, `hashCode`, `toString`, or `copy` when `data class` would generate them correctly. (ref: kotlin)"
  - "MUST expose read-only collection types (`List`, `Set`, `Map`) across module, package, or layer boundaries. NEVER expose `MutableList`, `MutableSet`, or `MutableMap` from a public API. (ref: kotlin)"
  - "NEVER catch generic `Exception`, `Throwable`, or `RuntimeException` in business logic. Catch the specific subtype, or use `runCatching { ... }` and handle the resulting `Result` with `onFailure` / `getOrElse`. (ref: kotlin)"
  - "MUST model branching domain outcomes with `sealed interface` or `sealed class` and exhaustive `when` (no `else` branch on a sealed hierarchy). NEVER use exceptions as control flow for expected business outcomes. (ref: kotlin)"
  - "MUST launch coroutines from a structured `CoroutineScope` tied to an explicit lifecycle. NEVER call `GlobalScope.launch` or `GlobalScope.async` in application code. (ref: kotlin)"
  - "MUST switch to `Dispatchers.IO` for blocking I/O inside suspending functions via `withContext(Dispatchers.IO) { ... }`. NEVER perform blocking JDBC, file, or network calls on `Dispatchers.Default` or `Dispatchers.Main`. (ref: kotlin)"
  - "MUST use Spring constructor injection (`class Foo(private val bar: Bar)`). NEVER use field injection (`@Autowired lateinit var ...` or `@Autowired` on mutable properties). (ref: kotlin)"
reminders:
  - "Before declaring a Kotlin variable → reach for `val` first; use `var` only if reassignment is required by control flow (ref: kotlin)"
  - "Before catching an exception → catch the specific subtype, never `Exception` / `Throwable` / `RuntimeException` in business logic (ref: kotlin)"
verification:
  - "[ ] No `!!` non-null assertions outside test code (`grep -RInE '!!' src/main`) (ref: kotlin)"
  - "[ ] No `GlobalScope.launch` / `GlobalScope.async` in application code (`grep -RIn 'GlobalScope' src/main`) (ref: kotlin)"
  - "[ ] No Spring field injection via `@Autowired lateinit var` (`grep -RInE '@Autowired[[:space:]]+lateinit' src/main`) (ref: kotlin)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
