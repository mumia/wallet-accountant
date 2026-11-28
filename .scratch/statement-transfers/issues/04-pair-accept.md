# 04: Pair Accept: two pending rows as one Transfer

**What to build:** Accepting a row with `transfer_account_id` (a statement Account) and `match_imported_transaction_id` Accepts both rows, records both legs and confirms their Match, in one database transaction. Spec: `../spec.md`, "Ingestion" (Accept as Transfer with `match_imported_transaction_id`), "Ledger: Transfers between statement Accounts" (`MatchRow`), stories 5 and 6.

**Blocked by:** 02

**Status:** resolved

- [x] Body checks before any row is read: `MATCH_WITHOUT_TRANSFER`, `MATCH_LEG_AND_ROW`
- [x] The paired row: pending, on `transfer_account_id`, not the same row, exact opposite amount and currency, otherwise `NOT_A_MATCHING_ROW`; no date window
- [x] The paired row's Booking month must be its Account's Open month: `month-closed` or `month-not-open-yet`, naming that Account
- [x] Both rows record `Accepted`, folds written in ID order; both legs booked through `BookTransfer` (`MatchRow`), Account folds in Account ID order; Match confirmed
- [x] Any refusal: nothing booked, both rows pending
- [x] The response is the row's own `ImportedTransaction`
- [x] Remove the 501 guard for `match_imported_transaction_id`
- [x] Consistency row: Accept as Transfer (paired row)
- [x] HTTP tests:
  - both statements imported, one Accept: both rows Accepted, both balances move, one `match_id`, nothing In transit;
  - refusals, each leaving both rows pending: paired row on another Account; Accepted, Discarded or an undecided Suspected duplicate; the same row; another amount; both match fields; paired row's month closed; not open yet;
  - two pair Accepts sharing one paired row: exactly one succeeds;
  - reversing either leg of a pair: the other stays booked and In transit, only the reversed leg's row goes pending, the Match gets 404

## Comments

2026-11-28, implementation notes:

- **Ingestion (`Accept`):** with `MatchRow`, the row's own state is checked first through the Decider without writing (so an Accepted row still says `imported-transaction-already-accepted` before anything about the paired row). Then `pairedRow` loads the paired row and refuses as `NOT_A_MATCHING_ROW` when it isn't found, is the row itself, isn't pending, isn't on `transfer_account_id`, or hasn't the exact opposite amount and currency. Then its month is checked against its Account's Open month, naming that Account. `acceptInIDOrder` then records `Accepted` on both rows in ID order. If the paired row changed between the check and the write, the Decider's refusal also becomes `NOT_A_MATCHING_ROW`.
- **The paired row's leg ID** is the existing `CounterpartLeg`: it is the leg recorded on the counterpart, which a pair Accept never also gives to a cash leg.
- **Ledger (`BookTransfer`):** a `MatchRow` case records both legs with `transferLeg` (each exactly as a plain Accept records its row, naming the other Account), in Account ID order (`inAccountOrder`, now shared with the cash case), then `ConfirmTransfer`. A `MatchRow` with a cash counterpart, or one not on the counterpart, is an internal error: Ingestion's checks make it unreachable.
- **`month-not-open-yet` now names `account_id`** when the month is another Account's, as `month-closed` already did. The contract's `MonthProblem.account_id` covers both.
- **Paired row's month closed:** not reachable over HTTP. A pending row can't be in a closed month, because an upload ignores rows there and a month can't close with pending rows. Like the row's own month-closed check, it is a backstop, and the HTTP test covers `month-not-open-yet` only. The Ledger checks the counterpart's month again when it books the leg, which catches a Close that ran meanwhile.
- **Tests** (`statement_transfer_integration_test.go`): `TestPairAcceptingTwoPendingRows`, `TestPairAcceptingIsRefused` (every refusal, plus a row that doesn't exist, each leaving every row pending and nothing booked), `TestTwoPairAcceptsForOneRowLetOnlyOneThrough` (sharing the paired row, and two crossing pair Accepts of the same two rows; run 8× without a deadlock or flake), `TestReversingOneLegOfAPairAcceptLeavesTheOtherInTransit`. `TestStatementTransferPiecesAnswerNotImplementedYet` is gone, with its last case.
