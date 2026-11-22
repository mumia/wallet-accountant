# 06: Void refused when an import touches a closed month; `void_blocked_by`

**What to build:** Void is refused with 409 `statement-import-touches-closed-month` when the import touches a closed month, and every Statement import lists `void_blocked_by`. Spec: `../spec.md`, "Ingestion around the Open month" (Void, `void_blocked_by`), stories 14, 15.

**Blocked by:** 05

**Status:** resolved

- [x] The touches-closed-month rule: the earliest Booking month of its rows in any state, or the first import's start month
- [x] 409 with `booking_month` (earliest closed month touched), `open_month` and the spec's copy, after the existing refusals
- [x] `void_blocked_by` on list, upload and void responses, in the order `NOT_LATEST`, `HAS_ACCEPTED_ROWS`, `TOUCHES_CLOSED_MONTH`; empty for a voided import
- [x] HTTP tests: a later import with rows in a closed month; the first import with a closed start month; rows that were ignored don't block; `void_blocked_by` with several reasons at once
- [x] Consistency map: Void row

## Comments

- 2026-11-22, implementation notes:
  - **Domain:** one method, `StatementImportState.VoidBlockedBy(VoidStatementImport)`, drives both the Decider's refusal and every response. `ClosedMonthTouched(openMonth, startMonth)` returns the earliest closed month touched, which is the earlier of the start month (only when the import has an Opening balance) and `EarliestBookingMonth`. `VoidStatementImport` gains `OpenMonth` and `StartMonth`. When they're empty nothing is touched, so existing callers are unaffected.
  - **Void:** reads the Open month through `AccountMonthsForShare` after `LockAccount`, which is the same order as upload, Keep and Discard. For the first import, the Opening-balance reversal then writes the same `accounts` row this transaction already holds `FOR SHARE`.
  - **Reads:** `List` makes one call to `ImportAccount` (no lock), one to `LatestImport`, and one grouped `AcceptedRows` query per Connection. Upload works it out from the import it just recorded. A voided import lists none.
  - **Copy:** the 409 always reads "This import has rows in January 2026, which is closed. Reopen January 2026 first.", including when only the first import's Opening balance is in that month.

