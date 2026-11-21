-- One Open month per Account (ADR-0024): the Account fold keeps its Open
-- month, and per Booking month its unreversed Transactions (live), which a
-- Reopen checks. A closed month locks instead of being flagged, so the flag
-- goes. The Account fold's schema version is bumped, so stored folds
-- rebuild with their Open month; the empty default only satisfies NOT NULL.
ALTER TABLE accounts ADD COLUMN open_month text NOT NULL DEFAULT '';
ALTER TABLE account_months ADD COLUMN live int NOT NULL DEFAULT 0;
ALTER TABLE account_months DROP COLUMN flagged;
