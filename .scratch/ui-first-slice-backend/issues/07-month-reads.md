# 07: Month reads: states, `reopen_blocked`, blockers and the months list

**What to build:** `AccountMonth` reports `NOT_OPEN_YET`, `reopen_blocked` and `unchecked_statement_imports`, and `GET …/accounts/{account_id}/months` lists every month. Spec: `../spec.md`, "Month and Account reads", stories 17–19.

**Blocked by:** 04, 05

**Status:** ready-for-agent

- [ ] `AccountMonth` built from `OpenMonth` and `Live`; 404 for a month before the start date's month
- [ ] `listAccountMonths`: range from the start month to max(current UTC month, latest non-Discarded row month), newest first; one grouped query for counts and one for unchecked imports
- [ ] HTTP tests: right after the Account is added (start month `OPEN`, the rest `NOT_OPEN_YET`); a future-dated row extends the range; a Discarded future row doesn't; `reopen_blocked` flips with an Accept in the Open month
- [ ] Interim values from ticket 01 replaced

## Comments
