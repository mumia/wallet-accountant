# 01: Contract: land the new `api/openapi.yaml`

**What to build:** land the contract this spec settled, so the UI can pin its `openapi.yaml` to the merge commit. `api/openapi.yaml`, `api/vacuum-ignore.yaml` and the `lint-api` change are already written on branch `ui-first-slice-backend`. What's left is to regenerate, compile and keep CI green with interim behaviour. Spec: `../spec.md`, "Contract" and "Delivery".

**Blocked by:** —

**Status:** resolved

- [x] `make generate` (ogen server and Bruno collection), with the generated code committed
- [x] Every new operation (`listAccountMonths`, `reopenAccountMonth`, `changeAccount`, `changeConnection`, `testCsvMapping`) is wired, answering 501 with a problem until its ticket lands
- [x] Interim values that pass response validation: `open_month` = the first month from the start date's month that isn't in `Closed`; `pending`/`suspected_duplicates` 0; `reopen_blocked` true; `unchecked_statement_imports` from the existing `MonthImports`; `void_blocked_by` empty; `rows_ignored_in_closed_months` 0; `state` from the interim `open_month`; `flagged` no longer emitted
- [x] Connection `name` is required on create, so it is recorded in `ConnectionCreated` and on the lookup row and returned (uniqueness and rename come in ticket 10); existing tests send names
- [x] `make lint` and `make test-integration` pass
- [x] After merge: annotated tag `v0.2.0` on the merge commit, then tell the UI repo
- [x] Do not change the contract's shape: anything that seems wrong is a comment on the spec

## Comments

Implementation notes (2026-11-21):

- **Generated code**: `make generate` regenerated `internal/platform/httpapi/oas` and `api/bruno` (new requests for the five operations).
- **409s**: the operations whose 409 is now `MonthConflict` return `*oas.MonthProblem`, built by `httpapi.MonthConflict(type, title, detail, bookingMonth, openMonth)`. Existing refusals pass empty months.
- **501s**: the stubs are in `internal/ledger/adapters/http/pending.go` (`changeAccount`, `listAccountMonths`, `reopenAccountMonth`) and `internal/ingestion/adapters/http/pending.go` (`changeConnection`, `testCsvMapping`). They return `httpapi.ErrNotImplemented`, which `Errors.NewError` turns into 501 `/problems/not-implemented`. Each ticket deletes its stub.
- **Open month**: `domain.AccountState.OpenMonth()` derives it from the Month closes (`domain.FirstOpenMonth`: the first month from the start date's month that isn't closed). The Account list does the same from `account_months.closed_at`. Ticket 02 replaces this with the fold field, keeping `OpenMonth()` as the accessor.
- **Month states**: `ledgerapp.AccountMonth` gains `State` (`MonthOpen`/`MonthClosed`/`MonthNotOpenYet`), `UncheckedImports` (from `MonthImports`) and `ReopenBlocked` (always true until ticket 04). `Flagged` is no longer written, though the domain still flags until ticket 02 deletes it.
- **Temporary values**: `Account.pending` and `Account.suspected_duplicates` are 0 until ticket 08. `void_blocked_by` is `[]` until ticket 06. `rows_ignored_in_closed_months` is 0 until ticket 05.
- **Connection name**: `CreateConnection.Name`, `ConnectionCreated.name` and `ConnectionState.Name`, trimmed and 1–100 characters (`ErrConnectionNameInvalid` → 422 `invalid-connection`). It is stored in the new `connections.name` column (migration 000016), and the Connection fold schema version is now 2. There is no uniqueness yet (ticket 10).
- **400 or 422**: the schema's `maxLength` gives 400 before the handler runs. The contract now says only a name of spaces gets 422 (Connection create and rename, Account rename).
- **Tests**: `flag_integration_test.go` is deleted, because `flagged` is no longer in the API and tickets 02–06 test the rules that replace it. Its `januaryBonus` moved to `void_integration_test.go`. Month tests expect `NOT_OPEN_YET` for months after the open one. Connection test bodies carry names, with added cases for trimming, a blank name and a too long name.
- **Checks**: `make lint` (golangci-lint 0 issues, vacuum 100) and `go test -tags integration ./...` pass.

2026-11-23: `v0.2.0` tagged on the contract merge commit (d0e5de4). `v0.3.0` tagged on 8e8e908, after every ticket in this slice landed; the contract's shape is unchanged since `v0.2.0`.
