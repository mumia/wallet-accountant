# 10: ADR, amendments and glossary

**What to build:** the documentation changes in the spec's "Documentation" section. This ticket can land any time after 01.

**Blocked by:** —

**Status:** ready-for-agent

- [ ] New ADR "Transfers as two Matched legs" (next number in `docs/adr/`)
- [ ] ADR-0018 amendment (Opening count; the cash consequence resolved)
- [ ] ADR-0024 amendment (cash takes later months, closes by a count with a date, and Reopen has no Open-month condition)
- [ ] `internal/ledger/CONTEXT.md`:
  - reword Posting;
  - add Cash Account, Opening count, Cash count, Account transfer, Transfer, Transfer leg, Counterpart Account and In transit;
  - update Statement balance and Internal Category
- [ ] `CONTEXT-MAP.md`: Accept as Transfer; Ledger owns transfer Matches
- [ ] Check that every consistency row from tickets 02–09 is present
