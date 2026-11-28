# 06: Hand-off note for `wallet-accountant-ui`

**What to build:** once tickets 02–04 are merged, write a note for the UI repo explaining what changed in the API since `v0.10.1`, so the UI's spec can be written from it. It goes in the UI repo's `.scratch/statement-transfers/issues/` as a `needs-triage` issue, following that repo's `docs/agents/issue-tracker.md`.

**Blocked by:** 02, 03, 04

**Status:** ready-for-agent

- [ ] **First, the breaking change:** `listTransferSuggestions` answers `{ legs, imported_transactions }`; the cash flow's read of `items` becomes `legs`. Don't deploy `v0.11.0` ahead of that UI change
- [ ] Every changed field and reason: `match_imported_transaction_id`, `ImportedTransaction.account_id`, optional `account_id` on suggestions, `ACCOUNT_KIND_NOT_SUPPORTED` (replacing `COUNTERPART_NOT_CASH`), `MATCH_LEG_AND_ROW`, `NOT_A_MATCHING_ROW`
- [ ] Examples: Accept alone (In transit), Accept with `match_transaction_id`, pair Accept, suggestions without `account_id`, a pair refused for the paired row's month
- [ ] The rules the UI must reflect:
  - statement Accounts (bank, card) as Transfer counterparts; no automatic leg for them;
  - no Transfer on, or to, a loan or crypto wallet Account, cash included (the UI offered cash Transfers on those rows before);
  - reversing one statement leg leaves the other In transit;
  - a leg in a closed month can still be Matched
- [ ] The tag to pin (`v0.11.0`, or the first release containing ticket 04)

## Comments
