-- Connection names: every Connection has one, set on create. Their
-- uniqueness within an Account comes with renaming (ticket 10 of
-- .scratch/ui-first-slice-backend). There are no Connections to keep, so
-- the empty default only satisfies NOT NULL; the Connection fold's schema
-- version is bumped, so stored folds rebuild with their name.
ALTER TABLE connections ADD COLUMN name text NOT NULL DEFAULT '';
