# 12: CSV mapping test endpoint

**What to build:** `POST …/accounts/{account_id}/csv-mapping-tests` reads a sample file with an unsaved mapping and returns the first 10 rows, the readable count and every unreadable line, recording nothing. Spec: `../spec.md`, "Connections and the CSV mapping" (Mapping test), stories 24, 25.

**Blocked by:** 11

**Status:** resolved

- [x] A collecting mode in the statement reader (readable rows and `RowError`s together); upload keeps refusing whole files
- [x] `Connections.TestMapping`: same mapping validation as save, Account currency, no start-date filter, same size limit as an upload
- [x] HTTP tests: a good file (10 rows plus the count), a mixed file, a fully unreadable file, an invalid mapping (422 `invalid-csv-mapping`), an empty file (422 `empty-statement-file`); nothing written to the event store (`eventstoretest` counter)

## Comments

- 2026-11-23, implementation notes:
  - **Domain:** `parseStatementFile` now wraps `collectStatementFile`, which returns readable rows and `RowError`s together; `ParseStatementFile`/`ReadStatement` still refuse the whole file with a `*RowsError`. `ReadSample(mapping, file, currency)` in `statementfile.go` returns a `Sample` (the first `SampleRows` = 10 rows, the readable count, every unreadable line), with the currency's decimal places checked and no start-date filter. A file with neither readable nor unreadable rows (empty, or only a header) is `ErrEmptyStatementFile`; a header missing a mapped column is a 200 with line 1 unreadable.
  - **App:** `Connections.TestMapping` reads the Account through a new narrow port, `AccountCurrencies` (`ImportAccount`, served by the existing `ingestionLedger`), so `NewConnections` takes it as a new argument. The file is read after the request transaction ends; nothing is executed against an aggregate.
  - **HTTP:** `mappingtest.go`; `pending.go` is deleted (it only held this stub). The file is read by `statementFile`, shared with upload, so the 16 MiB limit and its `statement-file-too-large` problem are the same.
  - **Tests:** `domain/sample_test.go`; `mappingtest_integration_test.go` (good file with Dec 2025 rows before the start date, every optional field, mixed, unreadable, invalid mapping, empty file, unknown Account). Each HTTP call checks the event store count is unchanged with a new `apitest.App.EventCount` (a `count(*)` over `event_store` through the reader pool) rather than the `eventstoretest` counter, which is a test aggregate, not a counter of events.
