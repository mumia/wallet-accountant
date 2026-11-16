# 06: Accounts

**What to build:** a User adds an Account with a name, a kind, an Account currency and a start date, and lists or opens the Household's Accounts. A new Account shows that it is awaiting its first Statement import and has no balance. Nobody can type an Opening balance. Spec: `../spec.md` stories 15–20; ADR-0004, ADR-0007, ADR-0018.

**Blocked by:** 04 (Households, membership, Internal Categories and Idempotency-Key)

**Status:** claimed

- [ ] `POST/GET /households/{household_id}/accounts` and `GET …/accounts/{account_id}`
- [ ] Account kinds are bank account, credit card, loan and crypto wallet. Cash Accounts are out of scope for this slice and are refused
- [ ] Account Decider holding kind, Account currency, start date and whether it awaits its first Statement import
- [ ] A new Account reports awaiting its first Statement import, with no balance
- [ ] No field or endpoint sets or edits an Opening balance
- [ ] Amounts are JSON decimals with `format: decimal`, paired with a currency code, using the currency's exact number of decimal places (ADR-0007)
- [ ] HTTP tests: add, list, get; refusing a cash or unknown kind; another Household gets 404
- [ ] Row-level security test for every table this ticket adds; Account fold registered for rebuild
- [ ] Consistency map row for Add Account matches the code
