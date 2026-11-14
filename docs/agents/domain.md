# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

This repo is **multi-context**.

- **`CONTEXT-MAP.md`** at the repo root lists the bounded contexts and how they relate. Read it first.
- **`internal/sharedkernel/CONTEXT.md`**: language every context shares (currencies, exchange rates, Match). Always read it.
- **`internal/<context>/CONTEXT.md`** for each context the topic touches: `households`, `ledger`, `ingestion`, `planning`.
- **`docs/adr/`**: read the ADRs that touch the area you're about to work in. All ADRs live here, including context-specific ones.
- **`docs/architecture/consistency.md`**: which effects of each command are immediate and which are deferred. Read it before adding or changing a command.

If a file doesn't exist yet, **proceed silently**. Don't flag its absence or suggest creating it upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## File structure

```
/
├── CONTEXT-MAP.md
├── docs/
│   ├── adr/                         ← all decisions, numbered
│   └── architecture/consistency.md
└── internal/
    ├── sharedkernel/CONTEXT.md
    ├── households/CONTEXT.md
    ├── ledger/CONTEXT.md
    ├── ingestion/CONTEXT.md
    └── planning/CONTEXT.md
```

A term belongs to the context that owns the concept. Other contexts refer to it by ID and use the same word. Only put a term in the shared kernel if every context uses it with exactly the same meaning.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `CONTEXT.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0007 (event-sourced orders), but worth reopening because…_
