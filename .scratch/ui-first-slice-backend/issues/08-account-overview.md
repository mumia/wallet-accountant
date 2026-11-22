# 08: Account overview: `open_month`, `pending`, `suspected_duplicates`

**What to build:** every `Account` carries its Open month and its pending and Suspected duplicate counts over every month, read with no query per Account. Spec: `../spec.md`, "Month and Account reads" (`Account`), story 16.

**Blocked by:** 02

**Status:** resolved

- [x] `open_month` from the fold
- [x] Ledger→Ingestion port: counts for one or all Accounts of the Household in one grouped query, in the request's transaction
- [x] An index on `imported_transactions (household_id, account_id, state)` if none covers it
- [x] HTTP tests: two Accounts with different counts in `listAccounts`; rows in a month not open yet are counted; an Accept updates the count immediately

## Comments

- 2026-11-22, implementation notes:
  - **`open_month`** was already read from the fold since ticket 02; nothing changed there.
  - **Port:** the Ledger's `Ingestion` port gains `UnresolvedRows(account *uuid.UUID)`: one grouped query (`GROUP BY account_id`) over pending and Suspected duplicate rows, for one Account or, with nil, every Account of the Household. `ledgerapp.Accounts` now takes the port; `List` calls it once with nil and `Get` with the Account, both in the request's transaction. `Add` keeps 0/0: a new Account has no rows. Ingestion's `MonthCounts` is renamed `RowCounts`, since it now counts whole Accounts too.
  - **Index:** migration `000019_account_overview` adds `imported_transactions_state_idx (household_id, account_id, state)`. The existing indexes lead with `(household_id, account_id)` but none has `state`. No new table, so no new RLS test.
  - **Tests:** `TestAccountsCarryTheirOpenMonthAndCounts` (`internal/ingestion/adapters/http/account_overview_integration_test.go`): two Accounts with different Open months and counts in `listAccounts`, each checked against `getAccount`; the first Account's February rows (a pending row and a Suspected duplicate, in a month not open yet) are counted; a second Account's counts move with its first import; an Accept takes one off the pending count immediately.
