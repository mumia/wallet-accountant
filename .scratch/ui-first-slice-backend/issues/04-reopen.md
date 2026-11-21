# 04: Reopen the latest closed month

**What to build:** `POST …/months/{booking_month}/reopen` takes back the latest Month close while the Open month has no Accepted Transactions. Spec: `../spec.md`, "Ledger: Account Decider and fold" (`ReopenMonth`), stories 7–9.

**Blocked by:** 03

**Status:** resolved

- [x] `ReopenMonth` command and `MonthReopened` event (registered in the codec), collecting every reason
- [x] `reopenAccountMonth` handler with `Idempotency-Key`, returning `AccountMonth`; 409 `month-reopen-refused` with `reasons`, `booking_month`, `open_month`, `accepted` and one sentence per reason in `detail`
- [x] Decider tests: each reason alone and together; Reopen after a reversal empties `Live[OpenMonth]`
- [x] HTTP tests: close January and February, Reopen February, Reopen January refused (not latest); Accept in March blocks Reopen of February; after Reopen the month closes again with its Statement balance
- [x] Consistency map: new Reopen month row

## Comments

- 2026-11-21, implementation notes:
  - **Decider:** `AccountState.ReopenRefusal(month)` returns every failed reason as a `*MonthReopenRefusedError` (nil when Reopen is allowed). `reopenMonth` uses it, and so does `accountMonthOf`, so `reopen_blocked` on every `AccountMonth` is now real instead of always true. `NOT_CLOSED` means the month has no Month close, so a month before the start date is also `NOT_CLOSED`. `NOT_CLOSED` and `NOT_LATEST_CLOSED` can't both hold, so "together" is tested as each of them with the Accepted count.
  - **Fold:** no schema version bump. `MonthReopened` adds no fold field, and stored folds stay valid.
  - **Copy:** `NOT_CLOSED` reads "March 2026 isn't closed, so it can't be reopened." The Accepted sentence is singular for one: "March 2026 has 1 Accepted Transaction. Reverse it before reopening February 2026."
  - **Tests:** the HTTP tests live in `internal/ingestion/adapters/http/reopen_integration_test.go`, next to the fixtures they share with Month close. After the Reopen, March's reversed Lunch is pending again and its Accept is refused as `month-not-open-yet`.

