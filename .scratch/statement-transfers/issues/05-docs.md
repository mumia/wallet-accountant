# 05: Docs: ADRs, glossaries and the context map

**What to build:** record the slice's decisions in the docs. Spec: `../spec.md`, "Documentation" and "Consistency".

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] ADR-0026 amendment: statement counterparts (no automatic leg; alone, Matched to a leg, or paired with a pending row); the reverse rule applied; Matching across a closed month allowed; Transfers only between `CASH`, `BANK_ACCOUNT` and `CREDIT_CARD`; the "can reuse the legs" consequence resolved
- [ ] ADR-0024 amendment: a close locks amounts, descriptions and Postings, not Match state
- [ ] `internal/ingestion/CONTEXT.md`, Accepted: two rows on two statement Accounts can be Accepted together as one Transfer's legs
- [ ] `internal/ledger/CONTEXT.md`, Transfer leg: between two statement Accounts, each leg comes from its own Account's statement
- [ ] `CONTEXT-MAP.md`, Ingestion → Ledger: Accept as Transfer to a statement Account, and the paired row
- [ ] `docs/architecture/consistency.md`: any row tickets 02–04 didn't write
- [ ] `.scratch/next-steps.md`: this slice moves to Done

## Comments
