-- Statement import rows outside the Open month (ADR-0024): an import counts
-- the rows it ignored in closed months, and keeps the earliest Booking
-- month (YYYY-MM) of the Imported transactions it recorded, NULL when it
-- recorded none, which says which months it touches. The Statement import
-- fold's schema version is bumped, so stored folds rebuild with both.
ALTER TABLE statement_imports ADD COLUMN rows_ignored_in_closed_months int NOT NULL DEFAULT 0;
ALTER TABLE statement_imports ADD COLUMN earliest_booking_month text;
