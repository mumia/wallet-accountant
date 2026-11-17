-- Households: the Invitation and User folds and their lookup rows
-- (ADR-0011, ADR-0017). They belong to the instance, not to a Household, so
-- they have no household_id and are admitted only in instance scope: while
-- app.household_id is empty (ADR-0014).

CREATE TABLE invitations (
    id             uuid        PRIMARY KEY,
    role           text        NOT NULL,
    code_hash      text        NOT NULL,
    expires_at     timestamptz NOT NULL,
    redeemed_by    uuid,
    redeemed_at    timestamptz,
    version        bigint      NOT NULL,
    schema_version int         NOT NULL
);

-- Lookup row: code hash -> Invitation, replaced on reissue.
CREATE TABLE invitation_codes (
    code_hash     text PRIMARY KEY,
    invitation_id uuid NOT NULL UNIQUE REFERENCES invitations (id)
);

CREATE TABLE users (
    id                        uuid    PRIMARY KEY,
    oidc_issuer               text    NOT NULL,
    oidc_subject              text    NOT NULL,
    invitation_id             uuid    NOT NULL,
    is_instance_administrator boolean NOT NULL,
    version                   bigint  NOT NULL,
    schema_version            int     NOT NULL
);

-- Lookup row: OIDC identity -> User. Its primary key is the backstop that
-- stops one identity from registering twice, a rule no single aggregate can
-- enforce.
CREATE TABLE user_identities (
    oidc_issuer  text NOT NULL,
    oidc_subject text NOT NULL,
    user_id      uuid NOT NULL UNIQUE REFERENCES users (id),
    PRIMARY KEY (oidc_issuer, oidc_subject)
);

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['invitations', 'invitation_codes', 'users', 'user_identities'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format($p$
            CREATE POLICY instance_scope_only ON %I
                USING (NULLIF(current_setting('app.household_id', true), '') IS NULL)
                WITH CHECK (NULLIF(current_setting('app.household_id', true), '') IS NULL)
        $p$, t);
    END LOOP;
END
$$;

GRANT SELECT, INSERT, UPDATE ON invitations, users TO wa_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON invitation_codes, user_identities TO wa_app;
