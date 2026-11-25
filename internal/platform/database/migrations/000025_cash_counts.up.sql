-- A cash Account's month closes by a Cash count: the amount counted and the
-- day it was counted are kept with its Month close, alongside the month-end
-- balance the count proves, stored as the Statement balance. Both are NULL
-- for a statement Account's month and for an open one. The Account fold's
-- schema version is bumped, so stored folds rebuild with them.
ALTER TABLE account_months ADD COLUMN counted_on date;
ALTER TABLE account_months ADD COLUMN cash_count numeric;
