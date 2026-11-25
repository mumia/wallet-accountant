# 07: Record a cash Transfer leg first, and Match it on Accept

**What to build:** `recordCashTransaction` with a non-cash `counterpart_account_id` records an In transit leg. When the bank row arrives, `listTransferSuggestions` offers that leg. Accept with `match_transaction_id` Matches it instead of creating a second cash leg. Spec: `../spec.md`, "Ledger: Transfers" (Recording a cash leg first, Accept as Transfer step 4), stories 16, 17, 19 and 20.

**Blocked by:** 06

**Status:** ready-for-agent

- [ ] `recordCashTransaction` with a non-cash counterpart records one leg, unmatched
- [ ] `listTransferSuggestions`, through a Ledger port query, returns legs that:
  - are unmatched and not reversed,
  - name the row's Account as counterpart,
  - have the opposite amount,
  - are dated within ±7 days of the row's Transaction date.

  Closest first. Delete the stub
- [ ] Accept with `match_transaction_id`: the leg is checked (`invalid-transfer`, `not-a-matching-leg`), no cash leg is created, and the Match is confirmed. There is no date window when confirming
- [ ] HTTP tests:
  - suggestion within 7 days, not at 8, and not for another amount;
  - Accept with match;
  - two concurrent Accepts on one leg: exactly one succeeds;
  - a month closes with a leg In transit, and `in_transit=true` lists it
