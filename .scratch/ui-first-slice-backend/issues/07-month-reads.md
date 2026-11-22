# 07: Month reads: states, `reopen_blocked`, blockers and the months list

**What to build:** `AccountMonth` reports `NOT_OPEN_YET`, `reopen_blocked` and `unchecked_statement_imports`, and `GET …/accounts/{account_id}/months` lists every month. Spec: `../spec.md`, "Month and Account reads", stories 17–19.

**Blocked by:** 04, 05

**Status:** resolved

- [x] `AccountMonth` built from `OpenMonth` and `Live`; 404 for a month before the start date's month
- [x] `listAccountMonths`: range from the start month to max(current UTC month, latest non-Discarded row month), newest first; one grouped query for counts and one for unchecked imports
- [x] HTTP tests: right after the Account is added (start month `OPEN`, the rest `NOT_OPEN_YET`); a future-dated row extends the range; a Discarded future row doesn't; `reopen_blocked` flips with an Accept in the Open month
- [x] Interim values from ticket 01 replaced

## Comments

- 2026-11-22, implementation notes:
  - **Port:** the Ledger's `Ingestion` port gains `AccountImports`. It runs one grouped query of pending and Suspected duplicate counts per Booking month over non-Discarded rows; the latest of those months extends the range. It runs one query of the non-voided waiting or failed imports, with their `earliest_booking_month`. `domain.AccountImports.Month(m)` rebuilds what `MonthImports` reports for one month, so every listed month equals its `getAccountMonth` (the HTTP tests check this for every month).
  - **Range:** from the start date's month to the latest of that month, the current UTC month (`tx.Now()`), and the latest non-Discarded row month. Including the start month covers a start date in the future.
  - **404:** `getAccountMonth` answers 404 for a month before the start date's month.
  - **Interim values:** `state` was already worked out from the fold's `OpenMonth` (ticket 02), `reopen_blocked` from `ReopenRefusal` (ticket 04), and `unchecked_statement_imports` from `MonthImports`. The only month-related interim left was the 501 on `listAccountMonths`. The Account's `pending`/`suspected_duplicates` are ticket 08.
  - **Imports recorded before migration 000018:** their `earliest_booking_month` is NULL until their folds are rebuilt (`make rebuild`, schema version 4), so the list leaves them out of `unchecked_statement_imports` until then. The single-month read still finds them through their rows.

