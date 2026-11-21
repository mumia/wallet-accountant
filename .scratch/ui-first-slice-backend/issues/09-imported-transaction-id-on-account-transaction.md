# 09: `imported_transaction_id` on `AccountTransaction`

**What to build:** each row of an Account's Transaction list names the Imported transaction whose Accept recorded it. Spec: `../spec.md`, "Month and Account reads" (`AccountTransaction`), story 20.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] Set from the Account Posting lookup rows; add the column and bump the Transaction fold schema version if they don't carry it
- [ ] HTTP tests: present on an Accepted row, absent on the Opening balance Transaction and on a reversal

## Comments
