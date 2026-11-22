# 09: `imported_transaction_id` on `AccountTransaction`

**What to build:** each row of an Account's Transaction list names the Imported transaction whose Accept recorded it. Spec: `../spec.md`, "Month and Account reads" (`AccountTransaction`), story 20.

**Blocked by:** 01

**Status:** resolved

- [x] Set from the Account Posting lookup rows; add the column and bump the Transaction fold schema version if they don't carry it
- [x] HTTP tests: present on an Accepted row, absent on the Opening balance Transaction and on a reversal

## Comments

- 2026-11-22, implementation notes:
  - **No migration, no fold bump:** `ledger_transactions` has carried `imported_transaction_id` since fold schema version 2, and the Account list query already joins it for the reversal links. The query now also selects `t.imported_transaction_id`; `app.AccountTransaction` gains `ImportedTransactionID`, and the handler sets it when it isn't nil. The contract had the field since ticket 01.
  - **Reversals:** `TransactionState.Reversal` doesn't copy the link, so a reversal names no Imported transaction. The reversed Transaction keeps naming its row even after the row goes back to pending.
  - **Tests:** `TestAccountTransactionsNameTheirImportedTransaction` (`internal/ingestion/adapters/http/account_transaction_link_integration_test.go`): two Accepted rows name their Imported transactions (one of them still after being reversed); the reversal and the Opening balance Transaction name none.
