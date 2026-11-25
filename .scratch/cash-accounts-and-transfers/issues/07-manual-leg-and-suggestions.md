# 07: Record a cash Transfer leg first, and Match it on Accept

**What to build:** `recordCashTransaction` with a non-cash `counterpart_account_id` records an In transit leg. When the bank row arrives, `listTransferSuggestions` offers that leg. Accept with `match_transaction_id` Matches it instead of creating a second cash leg. Spec: `../spec.md`, "Ledger: Transfers" (Recording a cash leg first, Accept as Transfer step 4), stories 16, 17, 19 and 20.

**Blocked by:** 06

**Status:** resolved

- [x] `recordCashTransaction` with a non-cash counterpart records one leg, unmatched
- [x] `listTransferSuggestions`, through a Ledger port query, returns legs that:
  - are unmatched and not reversed,
  - name the row's Account as counterpart,
  - have the opposite amount,
  - are dated within ±7 days of the row's Transaction date.

  Closest first. Delete the stub
- [x] Accept with `match_transaction_id`: the leg is checked (`invalid-transfer`, `not-a-matching-leg`), no cash leg is created, and the Match is confirmed. There is no date window when confirming
- [x] HTTP tests:
  - suggestion within 7 days, not at 8, and not for another amount;
  - Accept with match;
  - two concurrent Accepts on one leg: exactly one succeeds;
  - a month closes with a leg In transit, and `in_transit=true` lists it

## Comments

2026-11-25, implementation notes:

- **Manual leg** (`RecordCash` → `recordCashLeg`, `internal/ledger/app/cash.go`):
  - With `CashTransaction.Counterpart`, the cash checks run as before (cash Account, currency, zero, date). Then the counterpart is read:
    - another cash Account is `ErrCashToCashNotBuilt`, which HTTP answers with 501 until ticket 09;
    - otherwise `RecordTransferLeg` books one leg, In transit. Its own refusals (`SAME_ACCOUNT`, `UNKNOWN_ACCOUNT`, `CURRENCY_MISMATCH`) are 422 `invalid-transfer` with the reason.
  - The Ledger's `pending.go` is gone.
- **Accept with `match_transaction_id`** (`BookTransfer`):
  - The cash Account is checked first, through the new `checkCashCounterpart`, which is shared with suggestions.
  - `matchingLeg` then requires an In transit leg on the cash Account that names the row's Account, in its currency, with the opposite amount. Anything else is `NOT_A_MATCHING_LEG`; there is no date window.
  - Only the bank leg is booked, so only its Account's fold is written. `ConfirmTransfer` Matches the two legs.
  - `match_transaction_id` without `transfer_account_id` gets 422 `MATCH_WITHOUT_TRANSFER` in HTTP.
- **Concurrency:**
  - Two Accepts that Match one leg both write the leg's Transaction fold. The second fails its version check, and its retry finds the leg Matched: `NOT_A_MATCHING_LEG`.
  - The test runs two different rows (two ATM rows of 100.00) against one leg, and expects exactly one 200 and one 422.
- **Suggestions:**
  - `ImportedTransactions.TransferSuggestions` reads the row, then the Ledger port's `TransferSuggestions`.
  - In the Ledger, `Transactions.TransferSuggestions` checks the cash Account. Then `TransactionReader.TransferSuggestions` selects from `account_postings` joined to `ledger_transactions`:
    - unmatched, not reversed, naming the row's Account;
    - amount `= -row amount`, in the row's currency;
    - `transaction_date` within ±7 days (`SuggestionWindow`);
    - ordered by `abs(date difference)`, then date, then ID.
  - The legs are returned as `Transaction` items. Ingestion's `pending.go` is gone.
- **No contract change, no migration.**
- **Tests** (`internal/ingestion/adapters/http/manual_leg_integration_test.go`, on a statement with two ATM rows of 100.00 and a deposit of 3000.00):
  - Manual legs:
    - a manual leg is against Account transfer, In transit, and not automatic;
    - the suggestions for the deposit are the legs 2 and 7 days away, closest first. Not the one 8 days away, nor another amount, nor one naming another Account;
    - Accepting with the leg 8 days away Matches it, with no second cash leg.
  - Refusals:
    - `NOT_A_MATCHING_LEG` for another amount, another counterpart or an unknown leg;
    - `MATCH_WITHOUT_TRANSFER`;
    - suggestions for a bank Account (`COUNTERPART_NOT_CASH`) or an unknown one (`UNKNOWN_ACCOUNT`).
  - Two concurrent Accepts on one leg: exactly one succeeds.
  - A wallet's January closes with a leg In transit. `transfer_legs_in_transit` is 1, and `in_transit=true` lists it.
  - The 501 test now holds only a cash ↔ cash leg (ticket 09).
- **Consistency rows:** Record cash Transaction (as a leg), Accept as Transfer (matching a leg), and a new one, List transfer suggestions.
