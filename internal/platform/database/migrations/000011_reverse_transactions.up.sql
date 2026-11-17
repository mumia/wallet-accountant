-- Reversing Transactions (ADR-0018): the Transaction fold links a reversed
-- Transaction and its reversal both ways, and the Account transaction list
-- shows both links. Imported transactions gain the Discarded state, stored
-- as 'discarded' in the existing state column. No new table, so the
-- row-level security of 000008 still covers every row.
ALTER TABLE ledger_transactions ADD COLUMN reversal_of_transaction_id uuid;
ALTER TABLE ledger_transactions ADD COLUMN reversed_by_transaction_id uuid;
