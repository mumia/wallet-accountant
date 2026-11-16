# 15: Month status and Month close

**What to build:** a User sees each Booking month's status for an Account and closes it by entering the Statement balance. The close is refused, listing every reason at once, while anything is pending or an undecided Suspected duplicate, anything is Uncategorised, a relevant Balance check is waiting or failed, or the computed balance differs from the Statement balance. There is no balancing entry. Spec: `../spec.md` stories 53–61, 64, "Ledger: Account", "Cross-context calls: Month close"; ADR-0018.

**Blocked by:** 10 (Recategorise, reverse Transactions and Discard pending Imported transactions), 12 (Suspected duplicates, Keep, and the waiting Balance check)

**Status:** claimed

- [ ] `GET …/accounts/{account_id}/months/{booking_month}` returns open or closed, Statement balance, computed balance, pending count, undecided Suspected duplicate count, Uncategorised count and flagged
- [ ] `POST …/accounts/{account_id}/months/{booking_month}/close` with the Statement balance; months are decided by Booking date
- [ ] Ledger reads unresolved Imported transactions (pending and Suspected duplicate) and Balance check states through an Ingestion `app` port
- [ ] Close conditions, decided by the Account Decider: Uncategorised count zero; computed closing balance (Opening balance plus every month up to this one) equals the Statement balance exactly; no unresolved Imported transactions for the Account and month; no Statement import with rows in or before the month whose check is waiting or failed; month not already closed
- [ ] A refusal returns every failed condition together, with both amounts and the difference for a balance mismatch
- [ ] No endpoint records a balancing entry
- [ ] Decider tests: each condition singly and together
- [ ] HTTP tests: every rejection reason singly and together; a successful close; closing an already-closed month refused; another Household gets 404
- [ ] Consistency map row for Close month matches the code
