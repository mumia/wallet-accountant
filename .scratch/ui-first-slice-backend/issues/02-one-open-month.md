# 02: One Open month and sequential Month close

**What to build:** the Account fold holds `OpenMonth`. Close accepts only the Open month once it has ended (UTC), and moves `OpenMonth` on. Flagging is deleted. Spec: `../spec.md`, "Ledger: Account Decider and fold", "Clock", stories 1–4.

**Blocked by:** 01

**Status:** resolved

- [x] `OpenMonth` and `Live` on the fold, with a fold schema version bump
- [x] `CloseMonth` refusals: `ErrMonthNotOpenYet`, `ErrMonthAlreadyClosed` (month before Open), `ErrMonthNotEnded`; `ErrMonthNotStarted` removed
- [x] HTTP: 409 `month-not-open-yet` / `month-not-ended` with `MonthProblem` fields and the spec's copy
- [x] `FlagMonth`, `MonthFlagged`, `Flagged`, `flagged`/`flaggedFrom`, `FlagClosedMonths` (port and app) and the `account_months.flagged` column deleted (migration)
- [x] Decider tests: close in order; the last day versus the first day of the next month in UTC; December → January
- [x] HTTP tests: close January, then February opens; close February while January is open → 409
- [x] Consistency map: Close month row

## Comments

- 2026-11-21, implementation notes:
  - The migration is `000017_one_open_month.up.sql` only. The spec asked for a down/up pair, but every migration here is up-only (golang-migrate forward-only, ticket 02 of statement-import-to-month-close).
  - `OpenMonth` is stored on `accounts.open_month`, and `Live` on `account_months.live`. The Account fold schema version is now 6.
  - Close refusals are returned as `*domain.MonthError{Err, Month, OpenMonth}`, so the 409s carry `booking_month` and `open_month`. Ticket 03 can reuse it for `ErrMonthClosed`. `month-already-closed` now names both months too.
  - Copy helpers `httpapi.MonthWords` ("January 2026") and `httpapi.DateWords` ("1 December 2026").
  - Tests set the clock with `apitest.Start(t, apitest.WithClock(now))`, which goes through the new `wiring.Options.Now` to the event store's `tx.Now()`.
  - Consistency map: the Close month row is rewritten. Flagging is deleted, so the flag clauses of the Upload, Void, Accept, Set Transaction Category and Reverse rows are removed too. Their Open-month rules are left to tickets 03 to 06.

