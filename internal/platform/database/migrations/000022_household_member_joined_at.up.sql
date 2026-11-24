-- When each member joined a Household (.scratch/join-household, ticket 08),
-- for the members listing: the time of their household.memberadded event,
-- which the Household fold store writes from now on. Existing rows are
-- backfilled from the event store; wa_owner bypasses row-level security, so
-- the backfill sees every Household. A member who rejoined gets their latest
-- join.
ALTER TABLE household_members ADD COLUMN joined_at timestamptz;

UPDATE household_members m
SET joined_at = (
    SELECT max(e.time)
    FROM event_store e
    WHERE e.streamtype = 'households.household'
      AND e.subject = m.household_id
      AND e.type = 'io.walletaccountant.households.household.memberadded.v1'
      AND e.data ->> 'userid' = m.user_id::text
);

ALTER TABLE household_members ALTER COLUMN joined_at SET NOT NULL;
