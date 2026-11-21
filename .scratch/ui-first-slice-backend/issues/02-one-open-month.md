# 02: One Open month and sequential Month close

**What to build:** the Account fold holds `OpenMonth`. Close accepts only the Open month once it has ended (UTC), and moves `OpenMonth` on. Flagging is deleted. Spec: `../spec.md`, "Ledger: Account Decider and fold", "Clock", stories 1–4.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] `OpenMonth` and `Live` on the fold, with a fold schema version bump
- [ ] `CloseMonth` refusals: `ErrMonthNotOpenYet`, `ErrMonthAlreadyClosed` (month before Open), `ErrMonthNotEnded`; `ErrMonthNotStarted` removed
- [ ] HTTP: 409 `month-not-open-yet` / `month-not-ended` with `MonthProblem` fields and the spec's copy
- [ ] `FlagMonth`, `MonthFlagged`, `Flagged`, `flagged`/`flaggedFrom`, `FlagClosedMonths` (port and app) and the `account_months.flagged` column deleted (migration)
- [ ] Decider tests: close in order; the last day versus the first day of the next month in UTC; December → January
- [ ] HTTP tests: close January, then February opens; close February while January is open → 409
- [ ] Consistency map: Close month row

## Comments
