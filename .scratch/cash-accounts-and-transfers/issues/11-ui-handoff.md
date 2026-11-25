# 11: Hand-off note for `wallet-accountant-ui`

**What to build:** once tickets 02–09 are merged, write a note for the UI repo that explains what changed in the API since `v0.8.0`, so the UI's spec can be written from it. It goes in the UI repo's `.scratch/` as a `needs-triage` issue, or wherever that repo's issue tracker doc says.

**Blocked by:** 02, 03, 04, 05, 06, 07, 08, 09

**Status:** resolved

- [x] Every new and changed operation, field and problem type, with a short example request and response for each new flow:
  - add a cash Account;
  - record a cash Transaction;
  - close by count;
  - Accept as Transfer, with and without a suggestion;
  - reverse a leg
- [x] The rules the UI must reflect:
  - cash-only manual entry;
  - Opening count typed twice;
  - `counted_on`;
  - In transit legs;
  - which reversals cascade
- [x] Glossary terms the UI should use (Transfer, Transfer leg, In transit, Opening count, Cash count)
- [x] The tag to pin (the release that contains ticket 09)

## Comments

2026-11-25, implementation notes:

- **The note:** `../wallet-accountant-ui/.scratch/cash-accounts-and-transfers/issues/01-backend-hand-off.md`, `Status: needs-triage`, following that repo's `docs/agents/issue-tracker.md`. It isn't committed in the UI repo.
- **What it covers:**
  - every changed and new operation, field and problem type since `v0.8.0` (`0b6cc7b` is the only contract change since then), with the `invalid-transfer` reasons;
  - examples for adding a cash Account, recording a cash Transaction (plain, a leg noted first, and cash ↔ cash), closing by a count (refused with the gap, then passing), Accept as Transfer with and without a suggestion, and reversing a leg;
  - the rules: manual entry only for cash, the Opening count typed twice, `counted_on`, legs In transit not blocking a close, which reversals cascade, and that a leg's Category can't be changed;
  - the glossary terms, with "pending" flagged as Ingestion's word, not In transit's.
- **Tag to pin:** not tagged yet. The last tag is `v0.8.0`, and the spec planned `v0.9.0` for this contract's merge. The note asks for the first release cut after this branch merges.
- **Also noted for the UI:**
  - `acceptImportedTransaction`'s 422 schema is now `TransferProblem`;
  - every `month-closed` from Reverse now carries `account_id`.
