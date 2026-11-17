-- Month close (ADR-0018): the Account fold keeps, per Booking month, its
-- Month close (the Statement balance it closed with, when and by whom) and
-- whether it changed after it closed. A month is open while
-- statement_balance is NULL. No new table, so the row-level security of
-- 000008 still covers every row.
ALTER TABLE account_months ADD COLUMN statement_balance numeric;
ALTER TABLE account_months ADD COLUMN closed_at timestamptz;
ALTER TABLE account_months ADD COLUMN closed_by uuid;
ALTER TABLE account_months ADD COLUMN flagged boolean NOT NULL DEFAULT false;
