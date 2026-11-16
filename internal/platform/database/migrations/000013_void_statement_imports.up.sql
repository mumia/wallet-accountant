-- Voiding a Statement import (ADR-0018): the Statement import fold records
-- when it was voided. A voided import releases its file hash, so the file
-- is unique per Connection only among imports that aren't voided; the index
-- keeps its name, which the fold store recognises. Its rows become
-- 'discarded' in the existing state column. No new table, so the row-level
-- security of 000008 still covers every row.
ALTER TABLE statement_imports ADD COLUMN voided_at timestamptz;

DROP INDEX statement_imports_file_idx;
CREATE UNIQUE INDEX statement_imports_file_idx ON statement_imports (household_id, connection_id, file_hash)
    WHERE voided_at IS NULL;

-- A voided import's rows are found by import.
CREATE INDEX imported_transactions_import_idx ON imported_transactions (household_id, statement_import_id);
