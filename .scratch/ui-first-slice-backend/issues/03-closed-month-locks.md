# 03: A closed month locks its money

**What to build:** Accept, Keep, Discard and Reverse of anything booked in a closed month are refused. Recategorising to Uncategorised there is refused. Accept and Discard of a pending row in a month not open yet are refused, but a Suspected duplicate there can be Kept or Discarded. Spec: `../spec.md`, "Ledger: Account Decider and fold", "Ingestion around the Open month" (Keep, Discard, Accept), "Problem copy", stories 5, 6, 10, 11.

**Blocked by:** 02

**Status:** resolved

- [x] Ledger port method reading the Account's Open month with the fold row locked `FOR SHARE`
- [x] `BookTransaction`/`BookReversal`/`RecategoriseTransaction`/`ReopenOpeningBalance` refusals in the Decider
- [x] Keep/Discard/Accept month checks in Ingestion, in the spec's order
- [x] 409 `month-closed`, 409 `month-not-open-yet`, 422 `uncategorised-in-closed-month`, with the spec's copy
- [x] HTTP tests: each action in a closed month; a pending row in a month not open yet; a Suspected duplicate in a month not open yet kept and Discarded, after which its waiting check runs and the Open month can close
- [x] Consistency map: Accept, Keep, Discard, Set Transaction Category, Reverse Transaction rows

## Comments

- 2026-11-21, implementation notes:
  - **Port:** `AccountMonthsForShare` returns the start date and Open month, locking the `accounts` row `FOR SHARE`, and Keep and Discard use it. Accept doesn't: it reads the Open month through `ImportAccount` (which now carries `OpenMonth`) without a lock. Accept writes the Account row itself, so a share lock there would deadlock two concurrent Accepts on one Account. The Account's version check catches a Close that ran meanwhile, and the wiring maps the Ledger's refusal to Ingestion's `MonthError`.
  - **Ledger Decider:** `ErrMonthClosed` and `ErrUncategorisedInClosedMonth` are returned as `*MonthError`. Recategorising is refused only when moving *to* Uncategorised.
  - **Problem copy:** `httpapi.MonthClosed` and `httpapi.MonthNotOpenYet` build the 409s, and Month close now uses the second.
  - **Existing tests that Accepted or Discarded February's coffee while January was open** now use a January row, or close January first. `TestKeepingAndDiscardingTheLastSuspectedDuplicateConcurrently` now expects the Discard to be refused with `month-not-open-yet` when the Keep wins, because the kept row is pending in February.
  - **For ticket 05:** `TestAClosedMonthsImportedTransactionsAreLocked` gets a row into closed January by uploading after the close. Ticket 05 makes such rows ignored, which leaves no HTTP path to a pending or Suspected duplicate row in a closed month. That test will then need rewriting or dropping; the Ingestion check stays as a backstop.

