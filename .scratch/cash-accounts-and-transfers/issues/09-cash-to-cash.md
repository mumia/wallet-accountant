# 09: Transfer between two cash Accounts

**What to build:** `recordCashTransaction` with a cash `counterpart_account_id` records both legs (opposite amounts, same date, description and payee) and confirms their Match, in one command. Reversing either leg reverses both. Spec: `../spec.md`, "Ledger: Transfers", story 18.

**Blocked by:** 03, 05 (the Reverse cascade also needs 08)

**Status:** ready-for-agent

- [ ] Both Accounts must accept the date (no closed month on either). Folds are written in Account ID order
- [ ] Response shape as chosen in ticket 01
- [ ] Reversing either leg reverses both (extend 08's cascade)
- [ ] HTTP tests: record, see both legs and the Match; reverse one and both are reversed; a closed month on the counterpart is refused
