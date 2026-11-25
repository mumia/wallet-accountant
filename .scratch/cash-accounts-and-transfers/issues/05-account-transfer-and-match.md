# 05: Account transfer Category, Transfer legs and the Match aggregate

**What to build:** the building blocks of a Transfer, with no endpoint that uses them yet except `getMatch`:
- the Account transfer Internal Category;
- a Transaction that is a Transfer leg naming its counterpart Account;
- the Match aggregate linking two legs.

Spec: `../spec.md`, "Ledger: Transfers".

**Blocked by:** 01

**Status:** resolved

- [x] `RoleAccountTransfer` ("Account transfer"), seeded by Create Household. There is no migration
- [x] Accept and Set Category refuse it like any other Internal Category
- [x] `TransactionRecorded` gains `counterpartaccountid` and `createdbytransfer`. Counterpart checks: same Household, not itself, same currency (`invalid-transfer`)
- [x] `TransactionMatched` and `TransactionUnmatched`. Matching a Matched, reversed or non-leg Transaction is refused. Set Category on a leg is refused (`transfer-leg`)
- [x] Match stream: `match.confirmed` and `match.removed`, registered on the codec; `matches` fold table with row-level security; opposite amounts and crosswise counterparts checked
- [x] `getMatch` (a removed Match gets 404); delete the stub
- [x] `AccountTransaction` and `Transaction` return `counterpart_account_id`, `match_id` and `created_by_transfer`. `in_transit=true` filter; `AccountMonth.transfer_legs_in_transit`
- [x] Consistency row: Create Household
- [x] Decider tests for Transaction and Match; a row-level security adapter test for `matches`

## Comments

2026-11-25, implementation notes:

- **Category:** `RoleAccountTransfer` ("Account transfer") is the fourth Internal role, so Create Household seeds it. There is no migration. `chooseCategory` already refuses every Internal Category but Uncategorised, so Accept, Record cash Transaction and Set Category refuse it as `internal-category`.
- **Transaction Decider** (`internal/ledger/domain/transfer.go`):
  - `RecordTransaction` and `TransactionRecorded` gain `CounterpartAccountID` (`counterpartaccountid`) and `CreatedByTransfer` (`createdbytransfer`).
  - A leg must be exactly one Account Posting against one Category (otherwise `ErrInvalidPosting`), and must not name its own Account (`ErrCounterpartIsItself`). `CreatedByTransfer` without a counterpart is `ErrNotATransferLeg`.
  - `MatchTransaction` and `UnmatchTransaction` record `TransactionMatched` and `TransactionUnmatched` (`matchid`). Matching is refused for:
    - a Transaction that isn't a leg (`ErrNotATransferLeg`);
    - a reversed one (`ErrTransactionReversed`);
    - one that is already Matched (`ErrTransactionAlreadyMatched`).

    Unmatching is refused unless the leg has that Match.
  - `SetTransactionCategory` on a leg: `ErrTransferLeg`. The app also checks it before reading the Category, so a leg always gets `transfer-leg`, whatever the Category.
  - Helpers: `IsTransferLeg`, `InTransit`, and `Leg(id)`, which gives a Match the leg's Posting and counterpart.
  - A reversal of a leg doesn't copy the counterpart yet. Ticket 08 decides the Reverse rules.
- **Match Decider** (`domain/match.go`):
  - `ConfirmMatch { Kind, Legs [2]TransferLeg }` records `MatchConfirmed { kind, legs }`, and `RemoveMatch` records `MatchRemoved`.
  - The legs are refused with `ErrLegsDontMatch` when they are the same Transaction, have amounts that aren't opposite (or are zero), are in different currencies, or have counterparts that aren't crosswise.
  - `confirmed_by` and `confirmed_at` are the event's actor and time.
- **App** (`internal/ledger/app/transfers.go`):
  - The Match aggregate: stream `ledger.match`, events `io.walletaccountant.ledger.match.{confirmed,removed}.v1`, fold schema 1, and a rebuild target. The Transaction codec gains `…transaction.{matched,unmatched}.v1`, and the Transaction fold's schema goes to 4.
  - `Transactions.Match` serves `getMatch`; a removed Match is `ErrMatchNotFound`.
  - Building blocks for tickets 06, 07 and 09, which take the caller's transaction and have no caller yet:
    - `RecordTransferLeg` checks the counterpart: not itself (`SAME_ACCOUNT`), one of the Household's Accounts (`UNKNOWN_ACCOUNT`), the same currency (`CURRENCY_MISMATCH`). Each refusal is a `*TransferRefusedError` that unwraps to `ErrInvalidTransfer`. It then books the leg and records it against Account transfer.
    - `ConfirmTransfer(first, second)` confirms a new Match and marks both legs Matched. Legs that don't match, or one already Matched or reversed, are `NOT_A_MATCHING_LEG`.
    - Their HTTP mapping to `invalid-transfer` comes with the tickets that call them.
  - `MonthReader` (it was `CashCounts` in ticket 04) gains `LegsInTransit`, a count per Booking month that fills `AccountMonth.transfer_legs_in_transit`.
- **Migration `000026_transfers`:**
  - `ledger_transactions` gains `counterpart_account_id`, `created_by_transfer` and `match_id`, plus a partial index on the legs In transit;
  - a new `matches` fold table, with row-level security and grants. A removed Match keeps its row, with `removed` set.
- **HTTP:**
  - `getMatch` is implemented.
  - `AccountTransaction` and `Transaction` return `counterpart_account_id`, `match_id` and `created_by_transfer`, the last only when true.
  - `in_transit=true` filters the list, and `AccountMonth.transfer_legs_in_transit` is set.
  - Set Category on a leg: 422 `/problems/transfer-leg`.
  - The 501 stubs for `getMatch` and `in_transit` are gone.
- **Tests:**
  - Decider tests in `transfer_test.go`: a leg, an automatic leg, the refusals, Set Category on a leg, Match and Unmatch, the matching refusals, and Match confirm and remove with every way legs can fail to match.
  - Adapter tests:
    - row-level security on `matches`;
    - fold round-trips for a Match (confirmed, then removed) and for a leg (In transit, Matched, In transit again);
    - the Account fold now also round-trips a Cash count, which ticket 04 had missed.
  - HTTP tests in `transfers_integration_test.go`: Account transfer refused as a Category; with no Transfer, `in_transit` is empty, the month count is 0, ordinary Transactions carry no leg fields, and `getMatch` on an unknown ID gets 404.
  - The category tree tests and the new-Household test list Account transfer.
  - **Not covered over HTTP yet:** `getMatch` 200 and a removed Match's 404 need a Transfer the API can record, so ticket 06 covers them. The same goes for `RecordTransferLeg` and `ConfirmTransfer` end to end.
- **Consistency rows:** Create Household (four Internal Categories) and Set Transaction Category (refused for a leg).
