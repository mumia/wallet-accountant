-- Display names (.scratch/join-household, ticket 01): what other members of
-- a User's Households see. NULL for Users who registered before display
-- names existed and haven't set one. The User fold's schema version is
-- bumped, so stored folds refold from their events.
ALTER TABLE users ADD COLUMN display_name text;
