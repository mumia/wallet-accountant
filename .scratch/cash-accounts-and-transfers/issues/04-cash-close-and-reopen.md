# 04: Close a cash month by a count, and Reopen

**What to build:** a cash month closes with the amount counted and the date it was counted, which must be after the month's end and no later than today. The books up to that date must agree with the count, and the month-end balance this proves is stored as the Statement balance. Reopen of a cash month skips the Open-month Transactions condition. Spec: `../spec.md`, "Ledger: cash Accounts", stories 11–13.

**Blocked by:** 03

**Status:** resolved

- [x] `CloseMonth.CountedOn`:
  - required for cash and refused for non-cash;
  - `invalid-count-date` when it isn't after the month's last day, or is in the future
- [x] Balance at the end of `CountedOn` (`BalanceAt`): choose per-day sums in the fold or a read of the Account Posting lookup rows, and note the choice
- [x] Mismatch reported with the existing `month-close-refused` reason. `MonthClosed` gains `countedon` and `count`. `AccountMonth.counted_on` is returned
- [x] Cash Reopen without the Open month's Accepted-Transactions condition
- [x] Consistency rows: Close month (cash), Reopen month (cash)
- [x] Decider tests, plus an HTTP test: a short count is refused with the gap, a loss is recorded against an Expense Category, then the close passes

## Comments

2026-11-25, implementation notes:

- **`BalanceAt`: a read of the Account Posting lookup rows**, not per-day sums in the fold.
  - The app (`Months.Close`, `internal/ledger/app/months.go`) asks the new port `CashCounts.MovedAfter` for the sum of the Account's `account_postings` booked after the month's last day, up to and including `counted_on`. It passes this to the Decider as `CloseMonth.MovedByCountedOn`, and `AccountState.BalanceAt(month, moved)` adds it to `ClosingBalance(month)`.
  - Why: per-day sums would put one row per day in the fold store for every Account, for a figure only a cash close needs. Reversals are booked on the reversed Transaction's Booking date, so the sum of the rows is the sum of non-reversed Transactions.
  - Consistency: the adapter (`CashCounts` in `internal/ledger/adapters/postgres/accounts.go`) first locks the Account's fold row `FOR UPDATE`. A cash Transaction booked concurrently in the window either committed before the lock and is summed, or waits and then fails its Account version check and is retried.
- **Domain** (`month_close.go`): `CloseMonth` gains `CountedOn *time.Time` and `MovedByCountedOn`.
  - Refusals, in order after "not added": a cash Account without `CountedOn` (`ErrCountedOnRequired`), a statement Account with one (`ErrCountedOnNotForStatementAccount`), then the existing checks. These two come before "awaiting first Statement import", so a bank Account that awaits it still gets the 400.
  - `ErrInvalidCountDate` when `CountedOn` is in the month or before, or after today (UTC). It is checked after the month checks, so a month that hasn't ended still gets `month-not-ended`.
  - `MonthClosed` gains `countedon` (DateLayout) and `count`, both omitted for a statement Account. Its `statementbalance` is the proved month-end balance (`ClosingBalance(month)`), not the count. `MonthClose` gains `CountedOn` and `Count`.
- **Reopen** (`month_reopen.go`): `ReopenRefusal` doesn't count the Open month's Transactions for a cash Account. So `reopen_blocked` is false on a cash Account's latest closed month.
- **HTTP** (`months.go`): the 501 stub is gone.
  - `counted_on` missing for cash, or sent for a statement Account: 400 `/problems/invalid-month-closing`.
  - A bad date: 422 `/problems/invalid-count-date` (a `MonthCloseProblem`).
  - A gap: the existing 422 `month-close-refused`, `BALANCE_MISMATCH` with `computed_balance` (the balance at the end of `counted_on`), `statement_balance` (the count) and `difference`.
  - `AccountMonth.counted_on` is returned on a closed cash month.
- **Migration `000025_cash_counts`:** `account_months.counted_on date` and `cash_count numeric`, both NULL for statement Accounts and open months. The Account fold's schema version goes to 7.
- **Tests:**
  - Decider tests in `cash_month_test.go`: a count that agrees (counted on the 3rd, and on today), a short count with the gap, a count date on the last day, inside the month and after today, cash without a date, a statement Account with one, and a cash Reopen with Transactions in the Open month.
  - HTTP tests in `cash_months_integration_test.go`: a short count is refused with the gap, the loss is recorded against an Expense Category, and the close passes with the month-end balance and `counted_on`. Reopen with February Transactions, then close again with a new count. Plus every refusal.
  - `TestRecordingCashTransactions` and `TestRecountingAnOpeningCountIsRefused` now send `counted_on`. The 501 case for it is gone.
- **Consistency rows:** Close month (cash), Reopen month (cash).
