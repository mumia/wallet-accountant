# 01: Contract: cash Accounts and Transfers in `api/openapi.yaml`

**What to build:** write the contract the spec settled, so the UI can pin its `openapi.yaml` to the merge commit. Every new operation is wired up and answers 501 until its ticket lands. New fields on existing operations are accepted by the schema but refused with 501 until their ticket lands. Spec: `../spec.md`, "Contract".

**Blocked by:** —

**Status:** ready-for-agent

- [ ] `AccountKind` gains `CASH`; `AccountCreation` gains `opening_count` and `opening_count_confirmation`
- [ ] `MonthClosing.counted_on`; `AccountMonth.counted_on` and `transfer_legs_in_transit`
- [ ] `ImportedTransactionAcceptance` gains `transfer_account_id` and `match_transaction_id`
- [ ] `AccountTransaction` and `Transaction` gain `counterpart_account_id`, `match_id` and `created_by_transfer`; the Category's internal role gains `ACCOUNT_TRANSFER`
- [ ] `listAccountTransactions` gains `in_transit`
- [ ] New operations: `recordCashTransaction`, `recountOpening`, `getMatch`, `listTransferSuggestions`. Pick the cash ↔ cash response shape and note the choice here
- [ ] New problem types: `opening-count-mismatch`, `opening-count-locked`, `not-a-cash-account`, `invalid-transfer` (with `reason`), `transfer-leg`, `invalid-count-date`
- [ ] Descriptions updated: `addAccount`, `createConnection`, `closeAccountMonth`, `reopenAccountMonth`, `acceptImportedTransaction`, `changeTransaction`, `reverseTransaction`
- [ ] `make generate`, with the generated ogen code and the Bruno collection committed
- [ ] 501 stubs (`pending.go`) for each new operation, each naming its ticket
- [ ] `make lint` and `make test-integration` pass
- [ ] After merge: annotated tag `v0.9.0` on the merge commit
- [ ] Don't change the spec's shape: anything that seems wrong is a comment on the spec
