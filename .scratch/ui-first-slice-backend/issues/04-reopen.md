# 04: Reopen the latest closed month

**What to build:** `POST …/months/{booking_month}/reopen` takes back the latest Month close while the Open month has no Accepted Transactions. Spec: `../spec.md`, "Ledger: Account Decider and fold" (`ReopenMonth`), stories 7–9.

**Blocked by:** 03

**Status:** ready-for-agent

- [ ] `ReopenMonth` command and `MonthReopened` event (registered in the codec), collecting every reason
- [ ] `reopenAccountMonth` handler with `Idempotency-Key`, returning `AccountMonth`; 409 `month-reopen-refused` with `reasons`, `booking_month`, `open_month`, `accepted` and one sentence per reason in `detail`
- [ ] Decider tests: each reason alone and together; Reopen after a reversal empties `Live[OpenMonth]`
- [ ] HTTP tests: close January and February, Reopen February, Reopen January refused (not latest); Accept in March blocks Reopen of February; after Reopen the month closes again with its Statement balance
- [ ] Consistency map: new Reopen month row

## Comments
