# 02: Accept a row as a Transfer to a statement Account

**What to build:** Accept as Transfer takes a `BANK_ACCOUNT` or `CREDIT_CARD` counterpart. No leg is recorded on the counterpart: the row's leg stays In transit, or, with `match_transaction_id`, is Matched to an In transit leg on the counterpart. Loan and crypto wallet Accounts are refused on every Transfer path, cash included. Spec: `../spec.md`, "Ledger: Transfers between statement Accounts", stories 1–4, 7, 8, 12 and 13.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] The counterpart check covers every kind: cash as today; a statement counterpart records only the row's leg
- [ ] `ImportedTransfer` renamed fields (`Counterpart`, `CounterpartLeg`); `createdbytransfer` only ever set for a cash counterpart
- [ ] `match_transaction_id` with a statement counterpart: `matchingLeg` against the counterpart, no date window
- [ ] Kinds rule in the Transaction Decider's leg rules: a `LOAN` or `CRYPTO_WALLET` on either side refused; the app maps it to `ACCOUNT_KIND_NOT_SUPPORTED`, naming the Account (the leg's own first). Applies to Accept as Transfer and `recordCashTransaction`
- [ ] No start-date check for a statement counterpart (`BEFORE_START_DATE` stays cash-only)
- [ ] Remove the 501 guard for a statement `transfer_account_id`; remove `COUNTERPART_NOT_CASH` and its problem detail
- [ ] Consistency rows: Accept as Transfer (statement counterpart), Reverse Transaction (Transfer leg)
- [ ] Decider tests: a leg with a loan or crypto wallet on either side refused
- [ ] HTTP tests:
  - bank → card alone: bank balance moves, `in_transit=true` lists the leg; then card row Accepted with `match_transaction_id`: shared `match_id`, `getMatch` returns both, Category totals leave out Account transfer;
  - bank ↔ bank, the same flow;
  - across a closed month: the 31 March leg Accepted alone, March closed, the 2 April row Matched to it; reversing the April leg then unmatches the March leg, which stays In transit, and March stays closed;
  - reversing either leg leaves the other booked, In transit, and only the reversed leg's row pending;
  - a row dated before the counterpart's start date is Accepted, In transit;
  - `ACCOUNT_KIND_NOT_SUPPORTED` for: bank row with a loan counterpart, with a crypto wallet counterpart; loan row with a bank counterpart, with a cash counterpart; crypto wallet row with a cash counterpart; `recordCashTransaction` with a loan counterpart

## Comments
