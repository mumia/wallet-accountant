# 09: Transfer between two cash Accounts

**What to build:** `recordCashTransaction` with a cash `counterpart_account_id` records both legs (opposite amounts, same date, description and payee) and confirms their Match, in one command. Reversing either leg reverses both. Spec: `../spec.md`, "Ledger: Transfers", story 18.

**Blocked by:** 03, 05 (the Reverse cascade also needs 08)

**Status:** resolved

- [x] Both Accounts must accept the date (no closed month on either). Folds are written in Account ID order
- [x] Response shape as chosen in ticket 01
- [x] Reversing either leg reverses both (extend 08's cascade)
- [x] HTTP tests: record, see both legs and the Match; reverse one and both are reversed; a closed month on the counterpart is refused

## Comments

2026-11-25, implementation notes:

- **Recording** (`recordCashLeg`, `internal/ledger/app/cash.go`):
  - A cash counterpart now gets its own leg: the opposite amount, the same date, description and payee. Its counterpart is the request's Account, and it isn't flagged `created_by_transfer`.
  - Both legs go through `RecordTransferLeg` in Account ID order, then `ConfirmTransfer` Matches them.
  - Both IDs are made outside the retried transaction. `ErrCashToCashNotBuilt` is gone.
- **Closed month:** the counterpart's Account Decider refuses a closed month. The refusal comes back as an `*AccountError`, so `recordCashTransaction`'s 409 `month-closed` now names the counterpart in `account_id` (it used to always name the request's Account).
- **Response:** one `Transaction`, the leg on the request's Account, with `match_id`; `getMatch` names the other leg. This is ticket 01's choice.
- **Reverse cascade** (`cascades`, `internal/ledger/app/corrections.go`): the Matched leg is also reversed when it is an automatic cash leg (ticket 08), or when both legs are on cash Accounts. It reads both Account folds to tell.
- **No contract change, no migration.** The last 501 is gone, and so is its test (`pending_integration_test.go`).
- **Tests** (`internal/ledger/adapters/http/cash_transfers_integration_test.go`):
  - Wallet → purse:
    - the response is the wallet's −15.00 leg, Matched;
    - the purse has +15.00 on the same date, with the same Match;
    - `getMatch` names both legs, and both balances move.
  - Reversing the purse's leg reverses both legs, restores both balances, and the Match gets 404.
  - Into the purse's closed January: 409 naming the purse, and nothing is recorded.
- **Consistency rows:** Record cash Transaction (cash ↔ cash) and Reverse Transaction (Transfer leg).
