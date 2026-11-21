# 08: Account overview: `open_month`, `pending`, `suspected_duplicates`

**What to build:** every `Account` carries its Open month and its pending and Suspected duplicate counts over every month, read with no query per Account. Spec: `../spec.md`, "Month and Account reads" (`Account`), story 16.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] `open_month` from the fold
- [ ] Ledger→Ingestion port: counts for one or all Accounts of the Household in one grouped query, in the request's transaction
- [ ] An index on `imported_transactions (household_id, account_id, state)` if none covers it
- [ ] HTTP tests: two Accounts with different counts in `listAccounts`; rows in a month not open yet are counted; an Accept updates the count immediately

## Comments
