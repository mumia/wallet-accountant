# 03: A closed month locks its money

**What to build:** Accept, Keep, Discard and Reverse of anything booked in a closed month are refused. Recategorising to Uncategorised there is refused. Accept and Discard of a pending row in a month not open yet are refused, but a Suspected duplicate there can be Kept or Discarded. Spec: `../spec.md`, "Ledger: Account Decider and fold", "Ingestion around the Open month" (Keep, Discard, Accept), "Problem copy", stories 5, 6, 10, 11.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] Ledger port method reading the Account's Open month with the fold row locked `FOR SHARE`
- [ ] `BookTransaction`/`BookReversal`/`RecategoriseTransaction`/`ReopenOpeningBalance` refusals in the Decider
- [ ] Keep/Discard/Accept month checks in Ingestion, in the spec's order
- [ ] 409 `month-closed`, 409 `month-not-open-yet`, 422 `uncategorised-in-closed-month`, with the spec's copy
- [ ] HTTP tests: each action in a closed month; a pending row in a month not open yet; a Suspected duplicate in a month not open yet kept and Discarded, after which its waiting check runs and the Open month can close
- [ ] Consistency map: Accept, Keep, Discard, Set Transaction Category, Reverse Transaction rows

## Comments
