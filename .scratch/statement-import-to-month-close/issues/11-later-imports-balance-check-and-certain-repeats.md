# 11: Later imports: Balance check and certain repeats

**What to build:** a User uploads the next statement file for an Account with its Export balance, typed once. Rows already imported (same bank transaction ID, Booking date and amount) are skipped. The import is refused if the Account's Imported transactions, with the new rows, don't add up to the Export balance, showing the expected balance, the Export balance and the difference. Spec: `../spec.md` stories 38, 40, 43a, 43b, "Ingestion: Statement import: Later imports"; ADR-0018.

**Blocked by:** 08 (First Statement import: preview, confirm and Opening balance)

**Status:** claimed

- [ ] Later uploads refuse `preview_id`
- [ ] Balance check: Opening balance + every non-Discarded Imported transaction of the Account (pending, Accepted, kept, and this file's new rows) = Export balance. Ledger Transactions aren't used
- [ ] A failed check refuses the whole import with expected balance, Export balance and difference; a passing import is recorded with the check passed
- [ ] A row whose bank transaction ID, Booking date and amount all equal an earlier Imported transaction on the Account is skipped
- [ ] The import result reports rows imported, skipped as certain repeats, and ignored before the start date
- [ ] Decider tests: Balance check pass and fail, certain-repeat skipping
- [ ] HTTP tests: overlapping files with bank IDs skip repeats; Balance check pass and refusal; a wrong mapping caught by the check; the check still counts Accepted rows once Accept exists
- [ ] Consistency map row for Upload Statement import (later imports) matches the code

## Comments

Implementation notes (implementer agent):

- **Where things live.** The Statement import Decider (`internal/ingestion/domain/statementimport.go`) now records later imports: `RecordStatementImport` gains `RowsSkipped`, `OpeningBalance` and `EarlierTotal`; with `First: false` it checks Opening balance + earlier total + this file's rows total = Export balance and refuses with `*domain.BalanceCheckFailedError` (`errors.Is(err, domain.ErrBalanceCheckFailed)`; Expected, ExportBalance, Difference = Export − expected, Currency). `ErrLaterImportsNotSupported` is gone. `StatementImportRecorded` and the fold gain `RowsSkipped` (`rowsskipped`, omitted when 0, so older events read the same; fold schema version unchanged since 0 is right for them). Certain repeats: `domain.Statement.WithoutCertainRepeats([]domain.ImportedKey)` (pure; same bank ID, Booking date and amount by value; rows without a bank ID are never repeats; rows aren't compared with each other).
- **App.** `StatementImports.Upload`: a later import (Account not awaiting its first import) refuses any `preview_id` with 409 `/problems/opening-balance-already-recorded` (as before), takes a transaction-level advisory lock on the Account's imports (`StatementImportReader.LockAccount`, so two imports of one Account through any Connection check one after the other), reads `ImportedSoFar` (Opening balance = sum of `statement_imports.opening_balance` of the Account; total and keys of every `imported_transactions` row with `state <> 'discarded'`), skips repeats, and runs the Decider. It calls the Ledger port only for a first import. The Ledger port is unchanged: the check reads Ingestion's own folds, never Ledger Transactions.
- **Contract.** `StatementImport.rows_skipped_as_certain_repeats` (required int32). `StatementFileProblem` gains optional `expected_balance`, `export_balance`, `difference` (Money); 422 `/problems/balance-check-failed` uses them. `/problems/later-statement-imports-not-supported` is removed. `balance_check` stays `PASSED` only (refused imports aren't recorded; WAITING/FAILED come with ticket 12).
- **Migration** `000010_statement_import_certain_repeats`: `statement_imports.rows_skipped int NOT NULL DEFAULT 0`, and a partial index on `imported_transactions (household_id, account_id, bank_transaction_id)`. No new table, so no new RLS test.
- **Tests.** Decider: later import passing (event and fold), failing (error values; Opening balance and earlier total each needed); `WithoutCertainRepeats` (exact repeat, amount written differently, other date/amount/ID, no bank ID, identical rows within the file, totals). HTTP (`statementimports_later_integration_test.go`): overlapping file with bank IDs (1 imported, 2 skipped, 1 ignored, check passed, pending list, import list, Account balance untouched; a third repeat-only file); Balance check refusal with expected/Export/difference and nothing recorded, then the right Export balance passing; a wrong mapping (inverted sign) caught; `preview_id` and preview refused for later imports; rows without bank IDs never skipped. The old "later import → later-statement-imports-not-supported" subtest is removed.
- **Not done here: "the check still counts Accepted rows once Accept exists".** Accept (ticket 09) isn't on this branch, so no HTTP test can Accept a row. The reader counts every state except `'discarded'`, so Accepted rows count. Whoever merges 09 and 11 should add one HTTP test: first import, Accept a row, then a later import whose Export balance is right passes (and a wrong one is refused).
- **For tickets 10/12/14.** The literal `'discarded'` in `ImportedSoFar` must match the state value ticket 10 adds. Discarded rows are left out of both the total and the certain-repeat keys, so a voided import's rows don't block re-importing them (ticket 14); ticket 14 must also leave a voided first import's `opening_balance` out of the sum. Ticket 12 adds the waiting check for imports with Suspected duplicates; the lock and `ImportedSoFar` are where it plugs in.

Merger note (after merging ticket 09):

- **"The check still counts Accepted rows" is now covered.** `TestALaterImportsBalanceCheckCountsAcceptedRows` in `statementimports_later_integration_test.go` Accepts Rent and Salary of the first import, then uploads the overlapping file: a wrong Export balance is refused with 422 `/problems/balance-check-failed` (expected 2454.90, so Accepted rows are counted), and the right one passes with the Accepted Salary still skipped as a certain repeat. It passed first time; no code change was needed. Ticket 09 stores states `pending`/`accepted`, consistent with the reader's `state <> 'discarded'`.
