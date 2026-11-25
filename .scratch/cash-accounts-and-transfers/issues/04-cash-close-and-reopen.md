# 04: Close a cash month by a count, and Reopen

**What to build:** a cash month closes with the amount counted and the date it was counted, which must be after the month's end and no later than today. The books up to that date must agree with the count, and the month-end balance this proves is stored as the Statement balance. Reopen of a cash month skips the Open-month Transactions condition. Spec: `../spec.md`, "Ledger: cash Accounts", stories 11–13.

**Blocked by:** 03

**Status:** ready-for-agent

- [ ] `CloseMonth.CountedOn`:
  - required for cash and refused for non-cash;
  - `invalid-count-date` when it isn't after the month's last day, or is in the future
- [ ] Balance at the end of `CountedOn` (`BalanceAt`): choose per-day sums in the fold or a read of the Account Posting lookup rows, and note the choice
- [ ] Mismatch reported with the existing `month-close-refused` reason. `MonthClosed` gains `countedon` and `count`. `AccountMonth.counted_on` is returned
- [ ] Cash Reopen without the Open month's Accepted-Transactions condition
- [ ] Consistency rows: Close month (cash), Reopen month (cash)
- [ ] Decider tests, plus an HTTP test: a short count is refused with the gap, a loss is recorded against an Expense Category, then the close passes
