# 05: Statement import rows outside the Open month

**What to build:** upload splits off rows booked in closed months before anything else, and counts them. Rows in months not open yet import as pending. A failed immediate Balance check diagnoses the split-off rows that aren't in the books. Spec: `../spec.md`, "Ingestion around the Open month" (Upload), "Problem copy" (`balance-check-failed`), stories 10, 12, 13.

**Blocked by:** 03

**Status:** ready-for-agent

- [ ] Closed-month split before certain repeats and Suspected duplicates; `RowsIgnoredInClosedMonths` on `StatementImportRecorded` and the lookup row; `rows_ignored_in_closed_months` on the import (0 on the preview)
- [ ] The import's earliest Booking month stored on its lookup row (used by tickets 06 and 07)
- [ ] Diagnosis on an immediate failed check: `rows_in_closed_months_not_in_books` plus the months named in `detail`
- [ ] Domain tests: split before repeats; the diagnosis counts only rows that are neither repeats nor lookalikes
- [ ] HTTP tests: an overlapping file after January closed passes with its ignored count; a back-dated January row fails with the diagnosis; a file on "10 February" with January open leaves its February rows pending
- [ ] Consistency map: Upload row

## Comments
