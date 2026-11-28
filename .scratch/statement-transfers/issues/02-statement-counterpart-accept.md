# 02: Accept a row as a Transfer to a statement Account

**What to build:** Accept as Transfer takes a `BANK_ACCOUNT` or `CREDIT_CARD` counterpart. No leg is recorded on the counterpart: the row's leg stays In transit, or, with `match_transaction_id`, is Matched to an In transit leg on the counterpart. Loan and crypto wallet Accounts are refused on every Transfer path, cash included. Spec: `../spec.md`, "Ledger: Transfers between statement Accounts", stories 1–4, 7, 8, 12 and 13.

**Blocked by:** 01

**Status:** resolved

- [x] The counterpart check covers every kind: cash as today; a statement counterpart records only the row's leg
- [x] `ImportedTransfer` renamed fields (`Counterpart`, `CounterpartLeg`); `createdbytransfer` only ever set for a cash counterpart
- [x] `match_transaction_id` with a statement counterpart: `matchingLeg` against the counterpart, no date window
- [x] Kinds rule in the Transaction Decider's leg rules: a `LOAN` or `CRYPTO_WALLET` on either side refused; the app maps it to `ACCOUNT_KIND_NOT_SUPPORTED`, naming the Account (the leg's own first). Applies to Accept as Transfer and `recordCashTransaction`
- [x] No start-date check for a statement counterpart (`BEFORE_START_DATE` stays cash-only)
- [x] Remove the 501 guard for a statement `transfer_account_id`; remove `COUNTERPART_NOT_CASH` and its problem detail
- [x] Consistency rows: Accept as Transfer (statement counterpart), Reverse Transaction (Transfer leg)
- [x] Decider tests: a leg with a loan or crypto wallet on either side refused
- [x] HTTP tests:
  - bank → card alone: bank balance moves, `in_transit=true` lists the leg; then card row Accepted with `match_transaction_id`: shared `match_id`, `getMatch` returns both, Category totals leave out Account transfer;
  - bank ↔ bank, the same flow;
  - across a closed month: the 31 March leg Accepted alone, March closed, the 2 April row Matched to it; reversing the April leg then unmatches the March leg, which stays In transit, and March stays closed;
  - reversing either leg leaves the other booked, In transit, and only the reversed leg's row pending;
  - a row dated before the counterpart's start date is Accepted, In transit;
  - `ACCOUNT_KIND_NOT_SUPPORTED` for: bank row with a loan counterpart, with a crypto wallet counterpart; loan row with a bank counterpart, with a cash counterpart; crypto wallet row with a cash counterpart; `recordCashTransaction` with a loan counterpart

## Comments

2026-11-28, implementation notes:

- **The kind rule** is in the Transaction Decider (`checkLeg`): `RecordTransaction` gains `AccountKind` and `CounterpartKind`, which aren't recorded in the event, and a leg with any other kind than cash, bank or credit card (or none given) is refused as `*domain.AccountKindError`, its own Account first. `RecordTransferLeg` is the only path that records a leg; it passes both kinds and turns the refusal into `ACCOUNT_KIND_NOT_SUPPORTED` with an `*AccountError` naming the Account, which both the Ledger's and Ingestion's HTTP adapters already read for `account_id`.
- **Check order:** the counterpart's same-Account, Household and currency checks come before the kind rule, and the Account Decider books the leg before the Transaction Decider records it. So a loan row whose cash leg would land before the cash Account's start date gets `BEFORE_START_DATE`, not the kind reason. Both refuse; I left the order as it was.
- **`BookTransfer`:** `checkCashCounterpart` is no longer used by Accept; `checkCounterpart` returns the counterpart, and a non-cash one records only the row's leg (returning no Match), or Matches `MatchLeg`. `ImportedTransfer` and Ingestion's `TransferBooking` now say `Counterpart` and `CounterpartLeg`.
- **Still for ticket 03:** suggestions keep `checkCashCounterpart` and the `COUNTERPART_NOT_CASH` reason, which the Ingestion handler turns into a 501. A suggestions request for a loan therefore answers 501 rather than `ACCOUNT_KIND_NOT_SUPPORTED` until 03.
- **Reversing:** no code change; the existing cascade only reverses a `createdbytransfer` leg. Covered by `TestReversingOneStatementLegLeavesTheOtherInTransit` and the closed-month test.
- **Tests:** `statement_transfer_integration_test.go` (bank → card and bank ↔ bank, reversing either leg, Matching into and out of a closed month with a counterpart started after the row, every kind refusal) and a Decider table in `transfer_test.go`. The 02 case of `TestStatementTransferPiecesAnswerNotImplementedYet` is gone.

