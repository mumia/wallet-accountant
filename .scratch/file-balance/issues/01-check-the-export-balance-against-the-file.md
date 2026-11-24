# 01: Check the Export balance against the file's own balance

**What to build:** a CSV mapping can pick a **balance** column, a running balance per row. Every import (and first-import preview) through such a mapping checks that the typed Export balance equals the file's balance, meaning the balance on its newest row. A mismatch is refused before anything is recorded. With the file as the second witness, the first import asks for the Export balance once.

**Blocked by:** None (can start immediately)

**Status:** resolved

## Why

A User's first import of a Millennium BCP export was given the Export balance €1,000.00 instead of €8,758.21. The Opening balance came out as €3,495.15 instead of €11,253.36. Typing it twice and the preview didn't catch it, because the User believed the figure.

The file already said the right balance twice: in its header (`Saldo contabilístico:;8758.21`) and in its `Saldo` column (8,758.21 on the newest row). Nothing compares the typed Export balance with the file. On later imports a typo fails the Balance check, but the User can't tell a typo from a missing row.

## What to build

### Contract (`api/openapi.yaml`)

- [x] **`CsvColumns.balance`** (optional `CsvColumn`): "The balance after each row, as the bank works it out; lets every import check its Export balance against the file".
- [x] **`ParsedStatementRow.balance`** (optional `Money`): present when the mapping has a balance column.
- [x] **`CsvMappingTest.file_balance`** (optional): `{ amount: Money, line: int32 }`, the newest row's balance and its line. Or **`file_balance_problem`** (optional string): why it can't be worked out. One of the two when the mapping has a balance column, neither otherwise.
- [x] **`StatementImportPreviewUpload.export_balance_confirmation`** becomes optional. It is required only when the Connection's mapping has no balance column. A missing one then gets 422 `/problems/export-balance-mismatch`, as entries that differ do today. A mapping with a balance column ignores it.
- [x] **A new 422 problem for `previewStatementImport` and `uploadStatementImport`,** for example `/problems/export-balance-differs-from-file` (the name is the backend's call).
  - It carries `file_balance` (DecimalText) and `line`.
  - Detail: "The Export balance, €1,000.00, isn't the file's balance, €8,758.21 (line 14). Check it is the bank's booked balance."
  - When the file's balance can't be worked out, the import is refused: a separate problem, or the same one without `file_balance`, giving the reason.
- [x] The operations' descriptions say all of the above.
- [x] Every change is additive or relaxes a requirement. Release as the next minor version.

### Domain

- [x] The mapping's `balance` column is read with the mapping's decimal and thousands separators, like amounts, within the Account currency's decimal places. A row whose balance cell isn't a number is an unreadable row.
- [x] **The file's balance** is the balance on the newest row. The newest row is found from the balances, not from dates or a setting. With rows r1 … rn in file order:
  - if `rn.balance = r1.balance + sum(r2 … rn amounts)`, the rows are oldest first, and the file's balance is `rn.balance`;
  - if `r1.balance = rn.balance + sum(r1 … r(n-1) amounts)`, they are newest first, and it is `r1.balance`;
  - one row: that row's balance;
  - neither: the file's balance can't be worked out ("the balance column doesn't add up from one end of the file to the other").
  
  Every readable row the mapping reads counts, including rows the import will ignore (before the Account's start date, in closed months). The check is about the file.
- [x] **On preview and upload through a mapping with a balance column:**
  - a typed Export balance that isn't the file's balance is refused with the new problem;
  - a file whose balance can't be worked out is refused with its reason.
  
  These checks come after the file is read (unreadable rows first) and before the duplicate-file check, the Opening balance and the Balance check. Nothing is recorded.
- [x] Without a balance column nothing changes.

### ADR

- [x] Add a note to ADR-0018 ("The first Export balance gets extra protection"): when the Connection's mapping reads a balance column, the file's balance tests the first Export balance, so it is asked once. The preview stays.

### Tests

- [x] **Domain:**
  - the file's balance for oldest-first, newest-first and one-row files;
  - a column that doesn't add up;
  - a balance cell that isn't a number is an unreadable row;
  - rows before the start date still count.
- [x] **HTTP:**
  - preview and upload with the right balance, the wrong one (the problem with `file_balance` and `line`) and a column that doesn't add up;
  - a first import with a balance column and no confirmation passes; without a balance column, a missing confirmation is still refused;
  - the mapping test's `balance` per row, and its `file_balance` or `file_balance_problem`.

## Later

- Check each row's balance against the one before (`previous + amount`). That catches a missing or doubled row inside the file.
- Read a balance from a fixed cell outside the rows, such as a header line (`Saldo contabilístico`), for a bank whose rows have no balance.

## Then

The UI moves to the release. Its ticket is `../wallet-accountant-ui/.scratch/file-balance/issues/01-check-the-export-balance-against-the-file.md`.

## Comments

Implemented. Decisions the ticket left open:

- The new problems are `/problems/export-balance-differs-from-file` (with `file_balance` and `line`) and `/problems/file-balance-unknown` (the reason in `detail`).
- The detail writes amounts as the other refusals do, "1000.00 EUR", not "€1,000.00".
- A file whose balances add up from both ends to different balances (such as +10 then -10) can't tell its newest row, so it is refused as `file-balance-unknown` too.
- The mapping test gives `file_balance_problem` "the file has rows that can't be read" when some rows can't be read, since an import would refuse those first.
- Without a balance column, the duplicate-file check still comes before the file is read, so nothing changes for those mappings.
- Releasing as the next minor version (tag v0.7.0) is left to a human.
