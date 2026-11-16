-- Later Statement imports (ADR-0018): the Statement import fold counts the
-- rows it skipped as certain repeats, and an Account's Imported transactions
-- are looked up by bank transaction ID to find them.
ALTER TABLE statement_imports ADD COLUMN rows_skipped int NOT NULL DEFAULT 0;

CREATE INDEX imported_transactions_bank_id_idx ON imported_transactions (household_id, account_id, bank_transaction_id)
    WHERE bank_transaction_id <> '';
