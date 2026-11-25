# 01: Contract: cash Accounts and Transfers in `api/openapi.yaml`

**What to build:** write the contract the spec settled, so the UI can pin its `openapi.yaml` to the merge commit. Every new operation is wired up and answers 501 until its ticket lands. New fields on existing operations are accepted by the schema but refused with 501 until their ticket lands. Spec: `../spec.md`, "Contract".

**Blocked by:** —

**Status:** resolved

- [x] `AccountKind` gains `CASH`; `AccountCreation` gains `opening_count` and `opening_count_confirmation`
- [x] `MonthClosing.counted_on`; `AccountMonth.counted_on` and `transfer_legs_in_transit`
- [x] `ImportedTransactionAcceptance` gains `transfer_account_id` and `match_transaction_id`
- [x] `AccountTransaction` and `Transaction` gain `counterpart_account_id`, `match_id` and `created_by_transfer`; the Category's internal role gains `ACCOUNT_TRANSFER`
- [x] `listAccountTransactions` gains `in_transit`
- [x] New operations: `recordCashTransaction`, `recountOpening`, `getMatch`, `listTransferSuggestions`. Pick the cash ↔ cash response shape and note the choice here
- [x] New problem types: `opening-count-mismatch`, `opening-count-locked`, `not-a-cash-account`, `invalid-transfer` (with `reason`), `transfer-leg`, `invalid-count-date`
- [x] Descriptions updated: `addAccount`, `createConnection`, `closeAccountMonth`, `reopenAccountMonth`, `acceptImportedTransaction`, `changeTransaction`, `reverseTransaction`
- [x] `make generate`, with the generated ogen code and the Bruno collection committed
- [x] 501 stubs (`pending.go`) for each new operation, each naming its ticket
- [x] `make lint` and `make test-integration` pass
- [ ] After merge: annotated tag `v0.9.0` on the merge commit
- [x] Don't change the spec's shape: anything that seems wrong is a comment on the spec

## Comments

2026-11-25, implementation notes:

- **Decisions, now reflected in the spec:**
  - **Opening counts:** `opening_count` and `opening_count_confirmation` are `Money` objects, like every other amount in the contract, not decimal strings.
  - **Cash ↔ cash response:** `recordCashTransaction` always answers with one `Transaction`, the leg on the request's Account. For cash ↔ cash it carries `match_id`, and `getMatch` names the other leg.
  - **Account transfer Category:** `Category` has no internal role field, so no `ACCOUNT_TRANSFER` role was added. Account transfer shows as an `INTERNAL` Category.
- **Refusals with a reason:** a new response `TransferRefused` with schema `TransferProblem`, a Problem plus an optional `reason`, is the 422 of `acceptImportedTransaction`, `recordCashTransaction` and `listTransferSuggestions`. Its reasons are `CATEGORY_AND_TRANSFER`, `MATCH_WITHOUT_TRANSFER`, `COUNTERPART_NOT_CASH`, `SAME_ACCOUNT`, `CURRENCY_MISMATCH`, `UNKNOWN_ACCOUNT` and `NOT_A_MATCHING_LEG`. `httpapi.Unprocessable` builds one, and Accept's existing Category refusals now use it.
- **Other schema changes:** `MonthProblem` gains `account_id`. `AccountMonth.transfer_legs_in_transit` is required (0 until ticket 05). New schemas: `OpeningRecount`, `CashTransactionCreation`, `TransactionList`, `Match`, `MatchLeg`; new path parameter `MatchID`.
- **501s:**
  - **New operations:** stubs in `internal/ledger/adapters/http/pending.go` (`recountOpening`, `recordCashTransaction`, `getMatch`) and `internal/ingestion/adapters/http/pending.go` (`listTransferSuggestions`), each naming its ticket.
  - **New fields on existing operations:** a guard at the top of each handler answers 501:
    - a `CASH` kind or an Opening count on `addAccount` (02),
    - `counted_on` on close (04),
    - `in_transit=true` (05),
    - `transfer_account_id` or `match_transaction_id` on Accept (06, 07).
  - **Behaviour change:** a `CASH` Account now gets 501 instead of 422. Its case moved from `TestAddingAnAccountIsRefused` to the new `TestCashAndTransferOperationsAnswerNotImplementedYet`, which has one case per pending piece; each ticket deletes its own.
- **Checks:**
  - `make lint-go` (0 issues), `make lint-api` (vacuum 100) and `make test-integration` pass.
  - `make lint-chart` couldn't fetch kubeconform's schemas, because DNS for raw.githubusercontent.com failed on this machine. It's unrelated to this change.
