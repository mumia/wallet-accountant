# 06: Accept a bank row as a Transfer to a cash Account

**What to build:** Accepting an Imported transaction with `transfer_account_id` (a cash Account) does four things in one database transaction:
- records the bank leg, exactly as an Accept does today;
- records the cash leg automatically, dated on the row's Transaction date, with the opposite amount;
- confirms the Match between them;
- writes both Account folds in Account ID order.

Spec: `../spec.md`, "Ledger: Transfers" (Accept as Transfer), "Ingestion", stories 14, 15 and 22.

**Blocked by:** 03, 05

**Status:** resolved

- [x] Ledger port `BookTransfer`, called by Ingestion's Accept inside its transaction
- [x] Refusals (`invalid-transfer`):
  - `category_id` together with `transfer_account_id`;
  - a non-cash counterpart (`counterpart-not-cash`);
  - a different currency;
  - the row's own Account
- [x] A cash leg date in a closed cash month: `month-closed`, naming the cash Account
- [x] `match_transaction_id` is refused with 501 until ticket 07
- [x] Consistency row: Accept as Transfer
- [x] HTTP tests:
  - an ATM row: both balances move, both legs share `match_id`, and `getMatch` returns them;
  - a deposit in the other direction;
  - Category totals leave out Account transfer;
  - each refusal

## Comments

2026-11-25, implementation notes:

- **Ingestion** (`internal/ingestion/app/importedtransactions.go`):
  - `Accept` now takes an `Acceptance { Category, TransferAccount }`. With `TransferAccount`, it calls the Ledger port's new `BookTransfer` instead of `BookAccepted`, in the same transaction and after the same row checks.
  - The cash leg's ID is made once, outside the retried transaction, like the bank leg's.
  - New `ErrInvalidTransfer` and `*TransferRefusedError { Reason }`. `MonthError` gains `AccountID`.
- **Ledger** (`Transactions.BookTransfer`, `internal/ledger/app/transfers.go`):
  - The cash Account is checked in this order. Each refusal is a `*TransferRefusedError`:
    1. the row's own Account: `SAME_ACCOUNT`;
    2. not the Household's: `UNKNOWN_ACCOUNT`;
    3. not cash: `COUNTERPART_NOT_CASH`;
    4. another currency: `CURRENCY_MISMATCH`.
  - Then it records the two legs through `RecordTransferLeg`, in Account ID order, and confirms the Match through `ConfirmTransfer`.
  - `RecordTransferLeg` now also checks the leg's own Account (not found, currency). It wraps an Account Decider refusal in `*AccountError { Account }`, so the port can name the cash Account in a `month-closed`.
  - The cash leg is dated on the row's Transaction date, as both Booking and Transaction date, with no Value date.
- **Port** (`internal/wiring/ledgerport.go`): `BookTransfer` translates the Ledger's refusals into Ingestion's. `monthError` now fills `AccountID` from an `AccountError`, so a race that closes the bank month now also names the bank Account.
- **HTTP** (`internal/ingestion/adapters/http/importedtransactions.go`):
  - `category_id` with `transfer_account_id`: 422 `invalid-transfer`, `CATEGORY_AND_TRANSFER`, checked before anything else.
  - `match_transaction_id`: 501 until ticket 07.
  - The other refusals: 422 `invalid-transfer` with their reason.
  - `month-closed` carries `account_id` when the refusal names an Account.
- **No contract change, no migration.**
- **Tests** (`accept_transfer_integration_test.go`):
  - An ATM row Accepted as a Transfer to a wallet:
    - both balances move;
    - the bank leg is the row's Transaction against Account transfer, naming the wallet and linked to the row;
    - the cash leg is +1000.00 on the row's Transaction date, automatic, naming the bank;
    - both legs share a `match_id`, and `getMatch` returns both legs with `confirmed_by` and `confirmed_at`;
    - nothing is In transit.
  - A deposit the other way.
  - Every refusal reason, plus the 501 for `match_transaction_id`. Nothing is booked, and the rows stay pending.
  - A cash leg in the wallet's closed January: 409 `month-closed` naming the wallet, nothing booked.
  - The ledger's 501 test lost its Accept as Transfer case.
- **Category totals:** there is no Category totals read model yet. "Category totals, Budget actuals" are deferred effects not yet built. So the test checks that both legs are booked against the Internal Category Account transfer, which ADR-0004 keeps out of spending and income. When the totals are built, they should get that test.
- **Consistency row:** Accept as Transfer.
