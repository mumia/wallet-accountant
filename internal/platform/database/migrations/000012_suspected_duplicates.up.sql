-- Suspected duplicates (ADR-0018): an Imported transaction can be a
-- Suspected duplicate, stored as 'suspected_duplicate' in the existing state
-- column, and keeps the earlier Imported transaction it resembles. The
-- Statement import fold counts its Suspected duplicates, and its
-- balance_check can be 'waiting' or 'failed'. An import's undecided
-- Suspected duplicates are counted to know when its check runs. No new
-- table, so the row-level security of 000008 still covers every row.
ALTER TABLE imported_transactions ADD COLUMN resembles_imported_transaction_id uuid;
ALTER TABLE statement_imports ADD COLUMN suspected_duplicates int NOT NULL DEFAULT 0;

CREATE INDEX imported_transactions_suspected_idx ON imported_transactions (household_id, statement_import_id)
    WHERE state = 'suspected_duplicate';
