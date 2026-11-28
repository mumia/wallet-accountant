# 05: Docs: ADRs, glossaries and the context map

**What to build:** record the slice's decisions in the docs. Spec: `../spec.md`, "Documentation" and "Consistency".

**Blocked by:** 01

**Status:** resolved

- [x] ADR-0026 amendment: statement counterparts (no automatic leg; alone, Matched to a leg, or paired with a pending row); the reverse rule applied; Matching across a closed month allowed; Transfers only between `CASH`, `BANK_ACCOUNT` and `CREDIT_CARD`; the "can reuse the legs" consequence resolved
- [x] ADR-0024 amendment: a close locks amounts, descriptions and Postings, not Match state
- [x] `internal/ingestion/CONTEXT.md`, Accepted: two rows on two statement Accounts can be Accepted together as one Transfer's legs
- [x] `internal/ledger/CONTEXT.md`, Transfer leg: between two statement Accounts, each leg comes from its own Account's statement
- [x] `CONTEXT-MAP.md`, Ingestion → Ledger: Accept as Transfer to a statement Account, and the paired row
- [x] `docs/architecture/consistency.md`: any row tickets 02–04 didn't write
- [x] `.scratch/next-steps.md`: this slice moves to Done

## Comments

2026-11-28, implementation notes:

- **ADR-0026:** four `*Amended (statement-to-statement Transfers):*` bullets, in the style of the cash amendments: no automatic leg for a statement counterpart, with the three ways to Accept and no start-date check; the reverse rule applied; Matching across a closed month; only cash, bank and credit card Accounts. The "can reuse the legs" consequence is marked *Resolved*, and the In transit consequence names the pair Accept as a way a Match is confirmed.
- **ADR-0024:** one amendment bullet: a close locks amounts, descriptions and Postings, not Match state.
- **Glossaries and `CONTEXT-MAP.md`:** the spec's sentences, as written. The map's Ingestion → Ledger line no longer says only "cash legs" are suggested.
- **`consistency.md`:** nothing left to write. Tickets 02–04 wrote Accept as Transfer (paired row included), Reverse Transaction (Transfer leg), List transfer suggestions and the kind rule on Record cash Transaction.
- **`next-steps.md`:** the slice is under Done, with the warning not to deploy `v0.11.0` ahead of the UI. The file is untracked in git, so it's edited in place, not committed.

