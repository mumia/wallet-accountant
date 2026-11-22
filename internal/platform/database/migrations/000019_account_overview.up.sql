-- An Account's pending and Suspected duplicate counts are read with every
-- Account read, for one Account or all of a Household's in one grouped
-- query. The existing indexes on imported_transactions lead with
-- (household_id, account_id) but not state.
CREATE INDEX imported_transactions_state_idx ON imported_transactions (household_id, account_id, state);
