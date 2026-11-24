# 01: Skip footer lines in a CSV mapping

**What to build:** a CSV mapping can say how many lines at the bottom of a statement file to skip, as it already says for the top with `skip_leading_lines`. The mapping test also returns the last readable rows, so the User can check how the end of the file reads.

**Blocked by:** None (can start immediately)

**Status:** resolved

## Why

Some banks end their exports with footer lines: totals, a closing balance, a "generated on" line. Today a footer line is an unreadable row. That refuses the whole import with `/problems/unreadable-statement-file`, and the mapping test lists the footer under `unreadable_rows`. The only workaround is to edit the file by hand.

## What to build

### Contract (`api/openapi.yaml`)

- [ ] `CsvMapping.skip_trailing_lines`: `integer`, `int32`, `default: 0`. Description: "How many lines to skip at the end of the file, after the last row, such as totals or a closing balance; blank lines at the very end aren't counted".
  - It sits next to `skip_leading_lines`.
  - Existing mappings read as 0.
- [ ] `CsvMappingTest.last_rows`, required: the last 10 readable rows that aren't already in `rows`, in file order, with the same item schema as `rows`.
  - It is empty when the file has 10 or fewer readable rows, and has fewer than 10 when the file has 11–19.
  - `rows`, `readable_row_count` and `unreadable_rows` are unchanged.
- [ ] Neither change breaks a client. Release them in the next minor version (v0.6.0).

### Domain

- [ ] `CSVMapping.SkipTrailingLines` (`json:"skiptrailinglines,omitempty"`). It travels in `CSVMappingSaved`, so older events read as 0. No new event and no fold schema bump, as with `Encoding`.
- [ ] `Validate` refuses a negative value with `ErrInvalidCSVMapping`: "the number of trailing lines to skip must not be negative".
- [ ] In `parseStatementFile`, after `decode` and next to `skipLines`, drop the footer before CSV parsing:
  1. Drop the final line break, and any lines at the very end that are empty or only whitespace.
  2. Drop the last `SkipTrailingLines` physical lines.

  The CSV reader never sees the footer. Physical lines, as `skipLines` counts them, not CSV records.
- [ ] Line numbers are unchanged: file lines from 1, skipped lines at the top included.
- [ ] When the lines skipped at the top and bottom leave nothing to read, the result is the existing empty-file outcome (`/problems/empty-statement-file` on import, and the same for the mapping test). There is no new problem type.

### HTTP

- [ ] `mappingIn`/`mappingOut` carry `skip_trailing_lines`. Saving, listing and the mapping test all use it.
- [ ] The mapping test fills `last_rows`.
- [ ] Statement import (preview and upload) skips the trailing lines in the same way. There is no other change to import.

### Tests

- [ ] Domain tests (`statementfile_test.go`):
  - a file with a 2-line footer and `SkipTrailingLines: 2` reads every row and no footer;
  - with 0, it reports the footer lines as unreadable, with their file line numbers;
  - trailing blank or whitespace-only lines, and a missing final line break, don't change the count;
  - leading and trailing skips together;
  - skips that cover the whole file give no rows.
- [ ] Domain test (`connection_test.go`): a negative `SkipTrailingLines` is a mapping problem.
- [ ] HTTP tests:
  - save and list with `skip_trailing_lines`; a mapping saved without it reads 0;
  - the mapping test's `last_rows` with 5, 15 and 30 readable rows (empty, 5 rows, 10 rows) and its line numbers;
  - importing a file with a footer.

## Later

The closing balance is sometimes in the footer, and sometimes inside the rows (a balance column, or a balance row). Reading it to prefill or check the Export balance is a separate feature. It is not part of this ticket.

## Then

The UI moves to the release and offers the setting. Its ticket is `../wallet-accountant-ui/.scratch/csv-footer-lines/issues/01-skip-lines-at-the-bottom.md`.

## Comments

- Implemented on `main`. `make lint-go`, `make lint-api`, `go test ./...` and `make test-integration` pass.
- The trailing lines are dropped in `collectStatementFile` (shared by `parseStatementFile` and `ReadSample`), after `skipLines`, by `dropTrailingLines`.
- Blank or whitespace-only lines at the very end are now dropped even when `skip_trailing_lines` is 0. Before, a whitespace-only last line (such as `"  "`) was an unreadable row.
- The contract's `info.version` is still `0.1.0`, as before; v0.6.0 is the git tag to cut.
