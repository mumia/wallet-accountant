-- A Connection's name is unique within its Account, ignoring case (ticket
-- 10 of .scratch/ui-first-slice-backend). The index is the guard: the
-- Connection fold store maps its violation to connection-name-taken. There
-- are no Connections to keep, so no existing names are made unique first
-- (reset a dev database whose Account has two with one name).
CREATE UNIQUE INDEX connections_name_idx ON connections (household_id, account_id, lower(name));
