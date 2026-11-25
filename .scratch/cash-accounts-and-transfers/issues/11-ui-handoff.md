# 11: Hand-off note for `wallet-accountant-ui`

**What to build:** once tickets 02–09 are merged, write a note for the UI repo that explains what changed in the API since `v0.8.0`, so the UI's spec can be written from it. It goes in the UI repo's `.scratch/` as a `needs-triage` issue, or wherever that repo's issue tracker doc says.

**Blocked by:** 02, 03, 04, 05, 06, 07, 08, 09

**Status:** ready-for-agent

- [ ] Every new and changed operation, field and problem type, with a short example request and response for each new flow:
  - add a cash Account;
  - record a cash Transaction;
  - close by count;
  - Accept as Transfer, with and without a suggestion;
  - reverse a leg
- [ ] The rules the UI must reflect:
  - cash-only manual entry;
  - Opening count typed twice;
  - `counted_on`;
  - In transit legs;
  - which reversals cascade
- [ ] Glossary terms the UI should use (Transfer, Transfer leg, In transit, Opening count, Cash count)
- [ ] The tag to pin (the release that contains ticket 09)
