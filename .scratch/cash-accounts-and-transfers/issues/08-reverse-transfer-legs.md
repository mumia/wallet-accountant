# 08: Reversing a Transfer leg

**What to build:** reversing a Matched leg removes the Match and unmatches both legs.
- **Bank leg:** reversing it returns its row to pending and also reverses an automatic cash leg. A cash leg entered by hand stays, In transit.
- **Cash leg alone:** reversing it leaves the bank leg Accepted, In transit.
- **Closed months:** the whole command is refused if any Transaction it would reverse is in a closed month.

Spec: `../spec.md`, "Ledger: Transfers" (Reversing a Transfer leg), story 21.

**Blocked by:** 06 (the manual-leg case needs 07)

**Status:** ready-for-agent

- [ ] `RemoveMatch` and `TransactionUnmatched` on both legs, in Reverse's database transaction
- [ ] Cascade to a `createdbytransfer` leg. No cascade to a manual leg
- [ ] Closed-month check across every Transaction the command would reverse, before anything is recorded
- [ ] Update the consistency row: Reverse Transaction (Transfer leg)
- [ ] HTTP tests for each case in the spec's Testing Decisions ("Reversing")
