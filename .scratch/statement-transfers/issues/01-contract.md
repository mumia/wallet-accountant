# 01: Contract: Transfers between statement Accounts in `api/openapi.yaml`

**What to build:** write the contract the spec settled, so the UI can pin its `openapi.yaml` to the merge commit. The suggestions response shape changes, so the existing handler must answer the new shape in this ticket. Spec: `../spec.md`, "Contract" and "Delivery".

**Blocked by:** —

**Status:** resolved

- [x] `ImportedTransactionAcceptance` gains `match_imported_transaction_id`
- [x] `ImportedTransaction` gains a required `account_id`
- [x] `listTransferSuggestions`: `account_id` optional; answers the new `TransferSuggestionList` (`{ legs, imported_transactions }`, both required) instead of `TransactionList`
- [x] `TransferProblem.reason`: remove `COUNTERPART_NOT_CASH`; add `ACCOUNT_KIND_NOT_SUPPORTED` (with `account_id` naming the refused Account), `MATCH_LEG_AND_ROW` and `NOT_A_MATCHING_ROW`
- [x] Descriptions updated: `acceptImportedTransaction` (statement counterparts, no automatic leg, the three ways to Accept, the new reasons, month problems naming the paired row's Account), `listTransferSuggestions`, `recordCashTransaction` (loan and crypto wallet counterparts refused)
- [x] The existing suggestions handler answers `TransferSuggestionList`: today's legs as `legs`, `imported_transactions: []`
- [x] Every response carrying an `ImportedTransaction` fills `account_id`. If the read model lacks it, add the migration and backfill from the Statement import here
- [x] 501 guards, each naming its ticket:
  - a statement Account as `transfer_account_id` (02);
  - `account_id` left out of suggestions (03);
  - `match_imported_transaction_id` (04)
- [x] `make generate`, with the generated ogen code and the Bruno collection committed
- [x] `make lint` and `make test-integration` pass, the existing cash Transfer tests reading `legs`
- [ ] After merge: annotated tag `v0.11.0` on the merge commit
- [x] Don't change the spec's shape: anything that seems wrong is a comment on the spec

## Comments

2026-11-28, implementation notes:

- **No migration:** `imported_transactions` and the Imported transaction fold already carry `account_id`; only the API mapping was missing.
- **`TransactionList` removed:** `listTransferSuggestions` was its only user.
- **501s:** no `pending.go` this time; each guard returns `httpapi.ErrNotImplemented` naming its ticket:
  - a statement Account as `transfer_account_id` (02) and suggestions on one (03): the Ledger still refuses them as `COUNTERPART_NOT_CASH`, which the contract no longer has, so the handlers turn that reason into a 501 (`counterpartNotCash` in `importedtransactions.go`; ticket 02 deletes it);
  - suggestions without `account_id` (03);
  - `match_imported_transaction_id` (04), after the `MATCH_WITHOUT_TRANSFER` and `MATCH_LEG_AND_ROW` body checks, which are live now.
- **Tests:** `TestStatementTransferPiecesAnswerNotImplementedYet` has one case per pending piece; each ticket deletes its own. `TestImportedTransactionsNameTheirAccount` covers `account_id`. The manual-leg test reads `legs` and checks `imported_transactions` is empty.
- **Still allowed until ticket 02:** `recordCashTransaction` with a loan or crypto wallet counterpart, and Accept as Transfer from a loan or crypto wallet row to cash. The contract already says they're refused.

