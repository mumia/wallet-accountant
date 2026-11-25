# 10: ADR, amendments and glossary

**What to build:** the documentation changes in the spec's "Documentation" section. This ticket can land any time after 01.

**Blocked by:** —

**Status:** resolved

- [x] New ADR "Transfers as two Matched legs" (next number in `docs/adr/`)
- [x] ADR-0018 amendment (Opening count; the cash consequence resolved)
- [x] ADR-0024 amendment (cash takes later months, closes by a count with a date, and Reopen has no Open-month condition)
- [x] `internal/ledger/CONTEXT.md`:
  - reword Posting;
  - add Cash Account, Opening count, Cash count, Account transfer, Transfer, Transfer leg, Counterpart Account and In transit;
  - update Statement balance and Internal Category
- [x] `CONTEXT-MAP.md`: Accept as Transfer; Ledger owns transfer Matches
- [x] Check that every consistency row from tickets 02–09 is present

## Comments

2026-11-25, implementation notes:

- **ADR-0026, "Transfers as two Matched legs":** covers the legs against Account transfer and the Match, In transit, the automatic cash leg (and Matching a manual one instead), cash ↔ cash, and the Reverse cascade. Rejected: one Transaction with two Account Postings, and Accepting ATM rows as spending.
- **ADR-0018:** an "Amended (cash Accounts)" bullet covers the Opening count, typed twice and recountable while the start month is open, and gaps closed by recording what happened. The "How cash Accounts work is still open" consequence is marked resolved.
- **ADR-0024:** an "Amended (cash Accounts)" bullet: cash takes later months, up to today; it closes by a Cash count with its date; and Reopen has no Open-month condition.
- **`internal/ledger/CONTEXT.md`:**
  - added Cash Account, Opening count, Account transfer, Transfer, Transfer leg, Counterpart Account, In transit and Cash count;
  - reworded Posting, Opening balance (cash), Internal Category (it now lists Account transfer), Statement balance (cash) and Reopen (cash).
- **`CONTEXT-MAP.md`:** Ingestion → Ledger now describes Accept as Transfer and the suggestions query, and the Ledger owns transfer Matches.
- **Also `internal/sharedkernel/CONTEXT.md`:** Match now names the two Transfer legs of a Transfer.
- **Consistency rows:** all present, from tickets 02–09:
  - Add Account (cash), Recount opening, Create Connection;
  - Record cash Transaction, Accept as Transfer, List transfer suggestions;
  - Close month (cash), Reopen month (cash), Reverse Transaction (Transfer leg), Create Household.
