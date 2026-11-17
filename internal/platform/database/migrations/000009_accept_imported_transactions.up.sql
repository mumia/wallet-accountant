-- Accepting Imported transactions (ADR-0013): the Imported transaction fold
-- keeps the Ledger Transaction that booked it, the Transaction fold its
-- Imported transaction, and the Account fold counts, per Booking month, the
-- Transactions whose other side is in Uncategorised. No new table, so the
-- row-level security of 000008 still covers every row.
ALTER TABLE imported_transactions ADD COLUMN transaction_id uuid;
ALTER TABLE ledger_transactions ADD COLUMN imported_transaction_id uuid;
ALTER TABLE account_months ADD COLUMN uncategorised int NOT NULL DEFAULT 0;
