# Spec: Backend changes for the UI's first slice

Status: ready-for-agent

The Wallet Accountant UI (`../wallet-accountant-ui`, spec `.scratch/modern-frontend/spec.md`) decided its first slice against `api/openapi.yaml`. Doing so turned up backend changes. The biggest one replaces the first slice's "a closed month can still be changed, but any change is flagged" with **one Open month per Account**, and a closed month locks its money (ADR-0024). The rest serves the UI's overview without a call per Account, names Connections, adds an encoding and a test endpoint for the CSV mapping, and lets an Account be renamed. The UI's decisions (field names, shapes and the reasons behind them) are fixed. This spec settles what the UI left to the backend. It was decided with the user on 2026-11-21.

The contract is already written on branch `ui-first-slice-backend` (`api/openapi.yaml`, plus `api/vacuum-ignore.yaml`). Ticket 01 lands it. Every other ticket implements part of it, and must not change the contract without a comment on this spec.

## Problem Statement

A closed month that could still change silently stopped agreeing with the bank. A flag said so, but didn't say which later months now rested on a wrong number. Months closed in any order, so a later month's computed balance could be confirmed over an earlier month that was still moving. The UI's sidebar and Accounts overview needed a call per Account and per month just to know where each Account stands. A Connection had no name, so an Account with two banks' files showed two identical rows. A CSV mapping could only be tried by importing a statement, which a wrong mapping then refused, and files in Windows-1252 couldn't be read at all. A typo in an Account's name was permanent.

## Solution

Each Account has exactly one Open month. The start date's month opens first. The month after it opens only when the Open month closes, and a month closes only once it has ended. A closed month's amounts and descriptions are locked: Accept, Keep, Discard and Reverse are refused there, and Categories can still change, but never to Uncategorised. Months after the Open month are Not open yet. Statement import rows booked in them arrive as pending and wait, untouchable except for a Suspected duplicate decision. Rows booked in a closed month are ignored and counted. The Balance check formula is unchanged, so a row missing from a closed month shows up as a failed check that says which month to Reopen. Reopen takes back the latest Month close while the Open month has no Accepted Transactions. Void is refused while an import touches a closed month, and each import says beforehand why Void would be refused.

The Account carries its Open month and its pending and Suspected duplicate counts, and a new endpoint lists every month of an Account with its state. Connections get a unique name and can be renamed. The CSV mapping gets an encoding, and an endpoint reads a sample file with an unsaved mapping. Accounts can be renamed.

## User Stories

### One Open month

1. As a User, I want each Account to have one Open month, starting with its start date's month, so that I always know which month I'm working on.
2. As a User, I want the next month to open only when I close the Open month, December rolling into January, so that months are confirmed in order and each closing balance rests on confirmed months.
3. As a User, I want closing the month still in progress to be refused, so that I don't confirm a month the bank hasn't finished.
4. As a User, I want closing a month that isn't open yet refused, naming the Open month, so that I know which month to close first.
5. As a User, I want a closed month's amounts and descriptions locked, with Accept, Keep, Discard and Reverse there refused and saying the month is closed, so that a closed month stays what I confirmed.
6. As a User, I want to still change a Category in a closed month, but never to Uncategorised, so that I can explain spending better without undoing what the close checked.
7. As a User, I want to Reopen the latest closed month while the Open month has no Accepted Transactions, so that I can correct a month I closed too soon.
8. As a User, I want a refused Reopen to tell me every reason, including how many Accepted Transactions the Open month has, so that I know what to Reverse first.
9. As a User, I want a reopened month to be closed again with its Statement balance, and the month after it to be Not open yet again, so that nothing stays confirmed on a moved number.

### Statement imports around the Open month

10. As a User, I want rows booked in a month that isn't open yet to arrive as pending, visible but not Acceptable or Discardable, so that importing on 10 February doesn't lose February's first rows while January is still open.
11. As a User, I want to decide a Suspected duplicate even in a month that isn't open yet, so that its import's Balance check can run and the Open month can close.
12. As a User, I want rows booked in a closed month ignored and counted, like rows before the start date, so that overlapping files don't disturb closed months.
13. As a User, when a failed Balance check is caused by rows in a closed month that aren't in the books, I want the refusal to say so and name the month to Reopen, so that I'm not left guessing at a difference.
14. As a User, I want Void refused while an import touches a closed month, naming that month, so that voiding never changes a closed month.
15. As a User, I want each Statement import to list why Void would be refused right now, so that the UI can disable Void with the reason before I try.

### Overview reads

16. As a User, I want every Account to show its Open month and its pending and Suspected duplicate counts, so that the sidebar shows what's left to do without opening each Account.
17. As a User, I want to list every month of an Account, newest first, each with its state, counts and blockers, so that I can see the Account's history at a glance.
18. As a User, I want a month to say whether Reopen would be refused now, so that the UI disables Reopen before I try.
19. As a User, I want a month to list the Statement imports whose Balance check stops it closing before I try to close it, so that I can fix them first.
20. As a User, I want each row in an Account's Transaction list to name the Imported transaction it came from, so that the Reverse dialog can say that row goes back to To review.

### Connections and the CSV mapping

21. As a User, I want to name a Connection when I create it, uniquely within its Account, so that I can tell two banks' files apart.
22. As a User, I want to rename a Connection, so that a name chosen in a hurry isn't permanent.
23. As a User, I want to choose a file's encoding, UTF-8 or Windows-1252, so that files from banks that export Windows-1252 read correctly.
24. As a User, I want to test an unsaved mapping on a sample file and see the first rows as the server reads them, or every unreadable line with its reason, so that I know the mapping works before I import.
25. As a User, I want the mapping test to record nothing and not keep my file, so that trying a mapping is harmless.

### Accounts

26. As a User, I want to rename an Account, keeping its kind, currency and start date, so that a typo in its name isn't permanent.

## Implementation Decisions

### Contract

- The contract is already in `api/openapi.yaml`. It covers:
  - new operations: `listAccountMonths`, `reopenAccountMonth`, `changeAccount`, `changeConnection` and `testCsvMapping`
  - new schemas: `AccountMonthList`, `AccountChange`, `ConnectionName`, `ConnectionChange`, `CsvMappingTestUpload`, `CsvMappingTest`, `ParsedStatementRow`, `MonthProblem` and `MonthReopenProblem`
  - new response components: `MonthConflict` and `MonthReopenRefused`
  - new fields on `Account`, `AccountMonth`, `Connection`, `CsvMapping`, `StatementImport`, `StatementImportPreview`, `StatementFileProblem` and `AccountTransaction`
  - `AccountMonth.flagged` removed
- `date_format`, `decimal_separator` and `thousands_separator` are now `x-extensible-enum`, with their values as the bank file writes them. Zalando's UPPER_SNAKE_CASE rule [240] is waived for them in `api/vacuum-ignore.yaml`, which `make lint-api` passes to vacuum.
- `csv_mapping` in `CsvMappingTestUpload` is a plain multipart form field holding JSON text. ogen decodes it from the form values, not from file parts.
- `info.version` stays `0.1.0`.

### Problem copy

The UI shows a problem's `title` and `detail` verbatim and untranslated, and renders its extra fields by their presence. So these strings are User-facing copy, written in English. Months are written "January 2026", and dates "1 December 2026". All of these use `MonthProblem` unless stated otherwise.

| Type | Status | Title | Detail (example) | Fields |
|---|---|---|---|---|
| `/problems/month-closed` | 409 | Month closed | January 2026 is closed. Reopen it to change its entries. | `booking_month`, `open_month` |
| `/problems/month-not-open-yet` | 409 | Month not open yet | February 2026 isn't open yet. Close January 2026 first. | `booking_month`, `open_month` |
| `/problems/month-not-ended` | 409 | Month not ended | November 2026 can be closed from 1 December 2026. | `booking_month`, `open_month` |
| `/problems/uncategorised-in-closed-month` | 422 | Uncategorised in a closed month | January 2026 is closed, so its Transactions can't go back to Uncategorised. Pick another Category. | (plain `Problem`) |
| `/problems/month-reopen-refused` | 409 | Month can't be reopened | One sentence per reason, for example "February 2026 has 3 Accepted Transactions. Reverse them before reopening January 2026." or "Only December 2026, the latest closed month, can be reopened." | `MonthReopenProblem`: `reasons`, `booking_month`, `open_month`, `accepted` |
| `/problems/statement-import-touches-closed-month` | 409 | Statement import touches a closed month | This import has rows in January 2026, which is closed. Reopen January 2026 first. | `booking_month` (the earliest closed month it touches), `open_month` |
| `/problems/connection-name-taken` | 422 | Connection name taken | This Account already has a Connection named "Statement file". | (plain `Problem`) |
| `/problems/balance-check-failed` (existing) | 422 | (unchanged) | Unchanged, but when `rows_in_closed_months_not_in_books` > 0 it adds: "2 rows of this file fall in closed January 2026 and aren't in the books. Reopen January 2026, then import the file again." With several months, it names each. | adds `rows_in_closed_months_not_in_books` |

When a request breaks several rules, the refusal follows this order:
- **Accept**: a closed month, then a month not open yet, then the Imported transaction's own state.
- **Discard**: a closed month, then the state (a Suspected duplicate is allowed in a month not open yet), then a pending row in a month not open yet.
- **Keep**: a closed month, then the state.

### Clock

"The month has ended" and "the current calendar month" both use the server's clock in UTC (`tx.Now()`). There is no Household time zone (ADR-0024).

### Ledger: Account Decider and fold

- **New fold fields**:
  - `OpenMonth` (`MonthLayout`): set to the start date's month by `AccountAdded`.
  - `Live map[string]int`: the unreversed Transactions per Booking month. `TransactionBooked` adds one, and `ReversalBooked` takes one off the original's month. The reversal itself never counts.
  - `Closed` stays the map of Month closes. `MonthClose.Flagged` goes.
- **`CloseMonth`**:
  - Refused for a month before the start date (existing 422 `invalid-month-close`).
  - A month after `OpenMonth` gets `ErrMonthNotOpenYet`.
  - A month before `OpenMonth` gets `ErrMonthAlreadyClosed`.
  - The Open month gets `ErrMonthNotEnded` while `Today` (UTC) is before the first day of the next month.
  - `ErrMonthNotStarted` is removed, because those two cases cover it.
  - `MonthClosed` moves `OpenMonth` to the next month.
- **`ReopenMonth{Month}`** (new command): it collects every failed reason, the same way `CloseMonth` does:
  - `NOT_CLOSED`: the month isn't before `OpenMonth`.
  - `NOT_LATEST_CLOSED`: it is closed, but isn't the month just before `OpenMonth`.
  - `OPEN_MONTH_HAS_ACCEPTED_TRANSACTIONS`: `Live[OpenMonth] > 0`, carrying `OpenMonth` and the count.

  The event is `MonthReopened{Month}`, type `io.walletaccountant.ledger.account.monthreopened.v1` (follow the existing naming). It removes `Closed[Month]` and sets `OpenMonth` to it.
- **`BookTransaction`**: a Booking month before `OpenMonth` gets `ErrMonthClosed`, and one after it `ErrMonthNotOpenYet`. `RecordOpeningBalance` is unaffected: while the Account awaits its first import, `OpenMonth` is the start date's month, because Void of the first import is refused once the start month is closed.
- **`BookReversal`**: a Booking month before `OpenMonth` gets `ErrMonthClosed`.
- **`RecategoriseTransaction`**: moving to Uncategorised in a month before `OpenMonth` gets `ErrUncategorisedInClosedMonth`. Any other Category change in a closed month is allowed.
- **`ReopenOpeningBalance`**: refused with `ErrMonthClosed` when the start month is before `OpenMonth`. This is a backstop: Ingestion refuses first.
- **Deleted outright**, since there is no data to keep (decided 2026-11-21): `FlagMonth`, `MonthFlagged` and its event type registration, `flagged`/`flaggedFrom`, the Ingestion `Ledger` port's `FlagClosedMonths` and `ledgerapp.Months.FlagClosedMonths`, and the `account_months.flagged` column (a new down/up migration pair).
- The fold schema version is bumped, and `ClosingBalance` is unchanged.
- `AccountRenamed{Name}` (new event, type `…ledger.account.renamed.v1`) comes from a `RenameAccount` command. It trims the name, refuses a blank one with `ErrAccountNameRequired` and records nothing for the same name. The Account lookup row's name is updated.

### Ingestion around the Open month

- **The port**: the Ingestion `Ledger` port gains a method that reads an Account's `OpenMonth`, start date and closed months from its fold, **locking the fold row `FOR SHARE`** in the caller's transaction. A Close or Reopen of the same Account then serialises against a running Keep, Discard, upload or Void.
- **Keep**: refused with `month-closed` when the row's Booking month is before the Open month. A Suspected duplicate in a month not open yet can be kept.
- **Discard**: refused with `month-closed` before the Open month. In a month not open yet, it is refused with `month-not-open-yet` for a pending row and allowed for a Suspected duplicate. This exception exists because a Balance check waits for its import's Suspected duplicates (ADR-0018), and the Open month can't close while such a check waits. Locking those decisions would deadlock the Account (ADR-0024).
- **Accept**: the month check comes first in Ingestion (same problems), then the Ledger Decider enforces it again.
- **Upload**:
  - Rows whose Booking month is before the Open month are split off **first**, before certain repeats and Suspected duplicates are told. They are counted in `RowsIgnoredInClosedMonths`, a new field of `StatementImportRecorded` (`rowsignoredinclosedmonths`) that the import's lookup row also stores.
  - Rows in months not open yet are imported as usual.
  - The Balance check formula is unchanged.
  - When an immediate check fails, the split-off rows go through the repeat and lookalike tests against the earlier Imported transactions' keys. Those that match neither are counted in `rows_in_closed_months_not_in_books`, and their months are named in `detail`.
  - A waiting check that later fails records only its state, with no diagnosis.
  - The preview always reports 0, because a first import never meets a closed month.
- **Void**: refused with `statement-import-touches-closed-month` after the existing refusals (not latest, already voided, has Accepted rows). An import touches a closed month when either:
  - the earliest Booking month of any Imported transaction it recorded, in any state, is before the Open month, or
  - it has an Opening balance and the start month is before the Open month.

  The import's lookup row stores its earliest Booking month at record time, since rows are never added later.
- **`void_blocked_by`**: worked out per import on read, from the same rules: `NOT_LATEST`, `HAS_ACCEPTED_ROWS`, `TOUCHES_CLOSED_MONTH`, in that order. A voided import lists none.
- **Docs**: `docs/architecture/consistency.md` rows Upload, Void, Accept, Keep, Discard, Set Transaction Category, Reverse Transaction and Close month are rewritten to drop flagging and state the Open-month rules. A new Reopen month row is added.

### Month and Account reads

- **`AccountMonth`**:
  - `state` is worked out from `OpenMonth`.
  - `reopen_blocked` is false only for the month just before `OpenMonth` when `Live[OpenMonth] == 0`.
  - `unchecked_statement_imports` comes from the existing `MonthImports` (imports with rows in or before the month whose check is waiting or failed).
  - `closed_at` and `statement_balance` appear only on closed months.
  - `getAccountMonth` answers 404 for a month before the start date's month, and `NOT_OPEN_YET` for any later month.
- **`listAccountMonths`**:
  - It runs from the start date's month to `max(current UTC month, latest Booking month of a non-Discarded Imported transaction)`.
  - Counts and unchecked imports come from **one grouped query each** through the Ingestion port, not one per month. An import's earliest Booking month, stored on its lookup row, says which months it can block.
  - It is newest first and unpaged.
- **`Account`**:
  - `open_month` comes from the fold.
  - `pending` and `suspected_duplicates` come from a query through a Ledger→Ingestion port, inside the request's own transaction. It is always current, and `listAccounts` asks for every Account in **one grouped query**.
  - Add an index on `imported_transactions (household_id, account_id, state)` if none covers it.
- **`AccountTransaction.imported_transaction_id`**: set when an Accept recorded the Transaction. If the Account Posting lookup rows don't carry it, add the column and bump the Transaction fold schema version.

### Connections and the CSV mapping

- **Name**:
  - `ConnectionCreated` gains `name`, and the new `ConnectionRenamed{Name}` event (type `…ingestion.connection.renamed.v1`) comes from `RenameConnection`.
  - Names are trimmed and must be 1–100 characters, otherwise `ErrConnectionNameInvalid` → 422 `invalid-connection`. The same name again records nothing.
  - Uniqueness is guarded by a unique index on the Connection lookup rows, `(household_id, account_id, lower(name))`. Its violation maps to 422 `connection-name-taken`.
  - There is no data to keep, so the migration may assume no Connections exist (reset a dev database that has some).
- **Encoding**:
  - `CSVMapping` gains `Encoding` (`UTF_8` | `WINDOWS_1252`). Empty means `UTF_8`, and `CSVMappingSaved` writes it.
  - The file is decoded before it is parsed: a UTF-8 BOM is stripped as today, and Windows-1252 is decoded with a small stdlib table in the domain: bytes 0x80–0x9F map through the 27-entry Windows-1252 table, and every other byte maps to the same code point. The domain's depguard rule is strict, so `golang.org/x/text` can't be used there.
  - An unknown encoding is a mapping problem. The duplicate-file hash stays over the raw bytes.
- **Mapping test**:
  - `Connections.TestMapping(household, account, mapping, file)` validates the mapping exactly as a save does, then reads the whole file with the Account currency and no start-date filter.
  - It returns the first 10 readable rows, the readable count and every `RowError`.
  - Today's reader refuses the whole file on the first set of unreadable rows, so it gains a mode that collects readable rows and errors together. Upload keeps refusing whole files.
  - It has the same file-size limit as an upload. It records nothing, but declares `Idempotency-Key` like every POST (contract test).

### Delivery

- Ticket 01 (the contract) merges first, and its merge commit is tagged **`v0.2.0`** (decided 2026-11-21). The UI pins its `openapi.yaml` to that tag.
- The tag triggers `.github/workflows/release.yaml`, which publishes the image, the chart and a GitHub Release, so that release answers the new operations with 501 until their tickets land. That was accepted.

## Testing Decisions

- A good test checks behaviour through the public interface: Decider tests through `Decide`/`Evolve` (`decidertest`), application behaviour through HTTP integration tests (`apitest`, testcontainers PostgreSQL). It never checks internal structure. ogen validates every response against the contract, so each HTTP test also checks the contract.
- **Decider tests** (`internal/ledger/domain`):
  - `OpenMonth` moves forward and back.
  - Close refuses with not open yet, already closed and not ended (the last day of the month versus the first day of the next, in UTC).
  - Reopen collects every reason.
  - Book and reverse in a closed month or a month not open yet.
  - Recategorise to Uncategorised in a closed month; other Categories stay allowed.
  - `Live` counts across reversals.
  - Rename.
- **Ingestion domain tests**:
  - The closed-month split happens before repeats.
  - The diagnosis counts only unknown rows.
  - The Windows-1252 decode.
  - The collecting reader mode.
- **HTTP tests**, one per user story group:
  - the January → February → Reopen walk
  - importing on "10 February" with January open (a pending February row; Accept and Discard refused; a February Suspected duplicate kept or Discarded, then January closes)
  - an overlapping file after January closed (ignored count, the passing case)
  - a back-dated row (a failed check with the diagnosis)
  - Void refused with `TOUCHES_CLOSED_MONTH` (including the first import with a closed start month) and `void_blocked_by` on the list
  - `listAccountMonths` before and after the first import, with a future-dated row
  - `listAccounts` counts across two Accounts
  - Connection name uniqueness ignoring case, and rename
  - the mapping test with good, mixed and unreadable files, and an invalid mapping
  - Account rename
- **The clock**: tests set the clock through the existing time source in `internal/wiring/registry.go` (`time.Now` is injected). Never sleep.
- **Prior art**: `internal/ledger/domain/month_close_test.go`, `internal/ingestion/adapters/http/*_integration_test.go`, and ticket 16's `flag_integration_test.go`, which this work deletes or rewrites.

## Out of Scope

- A Household time zone.
- Bulk close, or closing a month that hasn't ended.
- Reopening more than the latest closed month in one step.
- Accepting into, or discarding pending rows in, a month not open yet.
- A diagnosis for a waiting Balance check that later fails.
- Cash Accounts (still open, ADR-0018).
- Bank feeds.
- Changing an Account's kind, currency or start date.
- Deleting Connections.
- Changing `Money.amount`, which stays a JSON number.
- `GET /currencies`, the `web` Zitadel app, `/api` and `/config.json`, which are already done or already decided (ADR-0021/0022).

## Further Notes

- Decided in the UI repo's tickets 07, 11, 15, 17 and 20 (`../wallet-accountant-ui/.scratch/modern-frontend/issues/`), and in the backend grilling on 2026-11-21. ADR-0024 is new, and ADR-0018 is amended. The Ledger and Ingestion glossaries and `CONTEXT-MAP.md` are updated.
- **Pushback the UI must take in (Q8 of the grilling):** a Suspected duplicate booked in a month not open yet can be Kept or Discarded. The UI showed such rows fully locked (UI ticket 14 amendment), and they need the duplicate decision.
- **Release tag:** `v0.2.0` on ticket 01's merge commit (see Delivery). `info.version` stays `0.1.0` until changed on purpose.
- The `wallet-accountant-deploy` change (ticket 14) lives in another repo and waits for the UI image.
