# 01: Contract: Transfers between statement Accounts in `api/openapi.yaml`

**What to build:** write the contract the spec settled, so the UI can pin its `openapi.yaml` to the merge commit. The suggestions response shape changes, so the existing handler must answer the new shape in this ticket. Spec: `../spec.md`, "Contract" and "Delivery".

**Blocked by:** —

**Status:** ready-for-agent

- [ ] `ImportedTransactionAcceptance` gains `match_imported_transaction_id`
- [ ] `ImportedTransaction` gains a required `account_id`
- [ ] `listTransferSuggestions`: `account_id` optional; answers the new `TransferSuggestionList` (`{ legs, imported_transactions }`, both required) instead of `TransactionList`
- [ ] `TransferProblem.reason`: remove `COUNTERPART_NOT_CASH`; add `ACCOUNT_KIND_NOT_SUPPORTED` (with `account_id` naming the refused Account), `MATCH_LEG_AND_ROW` and `NOT_A_MATCHING_ROW`
- [ ] Descriptions updated: `acceptImportedTransaction` (statement counterparts, no automatic leg, the three ways to Accept, the new reasons, month problems naming the paired row's Account), `listTransferSuggestions`, `recordCashTransaction` (loan and crypto wallet counterparts refused)
- [ ] The existing suggestions handler answers `TransferSuggestionList`: today's legs as `legs`, `imported_transactions: []`
- [ ] Every response carrying an `ImportedTransaction` fills `account_id`. If the read model lacks it, add the migration and backfill from the Statement import here
- [ ] 501 guards, each naming its ticket:
  - a statement Account as `transfer_account_id` (02);
  - `account_id` left out of suggestions (03);
  - `match_imported_transaction_id` (04)
- [ ] `make generate`, with the generated ogen code and the Bruno collection committed
- [ ] `make lint` and `make test-integration` pass, the existing cash Transfer tests reading `legs`
- [ ] After merge: annotated tag `v0.11.0` on the merge commit
- [ ] Don't change the spec's shape: anything that seems wrong is a comment on the spec

## Comments
