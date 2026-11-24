-- Invitations to a Household (.scratch/join-household, ticket 04): any
-- member issues one, and it lives in the Household's scope (ADR-0014). Only
-- its code lookup row stays in instance scope, so someone who doesn't know
-- the Household yet can redeem the code. The bootstrap Invitation stays in
-- the instance-scope invitations table, unchanged (ADR-0017).

CREATE TABLE household_invitations (
    household_id   uuid        NOT NULL,
    id             uuid        PRIMARY KEY,
    code_hash      text        NOT NULL,
    issued_by      uuid        NOT NULL,
    issued_at      timestamptz NOT NULL,
    expires_at     timestamptz NOT NULL,
    redeemed_by    uuid,
    redeemed_at    timestamptz,
    revoked_at     timestamptz,
    version        bigint      NOT NULL,
    schema_version int         NOT NULL
);

-- Listing a Household's open Invitations, oldest first.
CREATE INDEX household_invitations_household_idx ON household_invitations (household_id, issued_at);

ALTER TABLE household_invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE household_invitations FORCE ROW LEVEL SECURITY;

CREATE POLICY household_isolation ON household_invitations
    USING (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE ON household_invitations TO wa_app;

-- The code lookup row now points at either table, so it can no longer
-- reference invitations. household_id is the Invitation's Household, and
-- null for the bootstrap Invitation: redeeming sets the transaction to it.
ALTER TABLE invitation_codes DROP CONSTRAINT invitation_codes_invitation_id_fkey;
ALTER TABLE invitation_codes ADD COLUMN household_id uuid;
