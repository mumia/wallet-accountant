# 05: Statement import rows outside the Open month

**What to build:** upload splits off rows booked in closed months before anything else, and counts them. Rows in months not open yet import as pending. A failed immediate Balance check diagnoses the split-off rows that aren't in the books. Spec: `../spec.md`, "Ingestion around the Open month" (Upload), "Problem copy" (`balance-check-failed`), stories 10, 12, 13.

**Blocked by:** 03

**Status:** resolved

- [x] Closed-month split before certain repeats and Suspected duplicates; `RowsIgnoredInClosedMonths` on `StatementImportRecorded` and the lookup row; `rows_ignored_in_closed_months` on the import (0 on the preview)
- [x] The import's earliest Booking month stored on its lookup row (used by tickets 06 and 07)
- [x] Diagnosis on an immediate failed check: `rows_in_closed_months_not_in_books` plus the months named in `detail`
- [x] Domain tests: split before repeats; the diagnosis counts only rows that are neither repeats nor lookalikes
- [x] HTTP tests: an overlapping file after January closed passes with its ignored count; a back-dated January row fails with the diagnosis; a file on "10 February" with January open leaves its February rows pending
- [x] Consistency map: Upload row

## Comments

- 2026-11-22, implementation notes:
  - **Domain:** `Statement.WithoutClosedMonths(openMonth)` splits rows into `InClosedMonths` before `WithoutCertainRepeats` and `WithSuspectedDuplicates`. `Statement.ClosedMonthsNotInBooks(keys)` runs the split-off rows through the same repeat and lookalike index, which is now shared as `earlierIndex`. It returns `[]MonthRows` in month order. The command carries this list, and `BalanceCheckFailedError` carries it out only when the check fails.
  - **Lock:** later uploads read the Open month through `AccountMonthsForShare`, after `LockAccount`, which is the same order Keep and Discard use. First imports skip it, because they never meet a closed month.
  - **Lookup row:** migration 000018 adds `rows_ignored_in_closed_months` and `earliest_booking_month`. The latter is NULL when the import recorded no rows, so a month comparison in tickets 06 and 07 never matches it. The Statement import fold schema goes to 4. `StatementImportRecorded` gains `rowsignoredinclosedmonths` and `earliestbookingmonth`, both omitempty.
  - **Copy with several months:** "3 rows of this file fall in closed months and aren't in the books: 2 in January 2026 and 1 in February 2026. Reopen February 2026, then January 2026, then import the file again." It names the months latest first, because only the latest closed month reopens.
  - **Pre-existing:** the balance-check `detail` writes amounts without the currency's decimal places ("2480 EUR", "difference -5"). Left as is.
  - **Ticket 03's test:** `TestAClosedMonthsImportedTransactionsAreLocked` now checks only the Accepted Rent, which is refused as `month-closed`. No HTTP path brings a pending or Suspected duplicate row into a closed month any more.

