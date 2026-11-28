# 04: Pair Accept: two pending rows as one Transfer

**What to build:** Accepting a row with `transfer_account_id` (a statement Account) and `match_imported_transaction_id` Accepts both rows, records both legs and confirms their Match, in one database transaction. Spec: `../spec.md`, "Ingestion" (Accept as Transfer with `match_imported_transaction_id`), "Ledger: Transfers between statement Accounts" (`MatchRow`), stories 5 and 6.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] Body checks before any row is read: `MATCH_WITHOUT_TRANSFER`, `MATCH_LEG_AND_ROW`
- [ ] The paired row: pending, on `transfer_account_id`, not the same row, exact opposite amount and currency, otherwise `NOT_A_MATCHING_ROW`; no date window
- [ ] The paired row's Booking month must be its Account's Open month: `month-closed` or `month-not-open-yet`, naming that Account
- [ ] Both rows record `Accepted`, folds written in ID order; both legs booked through `BookTransfer` (`MatchRow`), Account folds in Account ID order; Match confirmed
- [ ] Any refusal: nothing booked, both rows pending
- [ ] The response is the row's own `ImportedTransaction`
- [ ] Remove the 501 guard for `match_imported_transaction_id`
- [ ] Consistency row: Accept as Transfer (paired row)
- [ ] HTTP tests:
  - both statements imported, one Accept: both rows Accepted, both balances move, one `match_id`, nothing In transit;
  - refusals, each leaving both rows pending: paired row on another Account; Accepted, Discarded or an undecided Suspected duplicate; the same row; another amount; both match fields; paired row's month closed; not open yet;
  - two pair Accepts sharing one paired row: exactly one succeeds;
  - reversing either leg of a pair: the other stays booked and In transit, only the reversed leg's row goes pending, the Match gets 404

## Comments
