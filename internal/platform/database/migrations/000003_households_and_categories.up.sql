-- Households: the Household fold with its membership, and the Ledger's
-- Category fold, which a new Household's Internal Categories are seeded into
-- (ADR-0003, ADR-0011). Every table here but user_households is
-- Household-scoped: it has household_id, forced row-level security and the
-- standard policy (ADR-0014).

CREATE TABLE households (
    household_id   uuid   PRIMARY KEY,
    name           text   NOT NULL,
    base_currency  text   NOT NULL,
    version        bigint NOT NULL,
    schema_version int    NOT NULL
);

-- Lookup row of the Household fold: one per member.
CREATE TABLE household_members (
    household_id uuid NOT NULL REFERENCES households (household_id),
    user_id      uuid NOT NULL,
    PRIMARY KEY (household_id, user_id)
);

CREATE TABLE categories (
    household_id   uuid   NOT NULL,
    id             uuid   PRIMARY KEY,
    name           text   NOT NULL,
    kind           text   NOT NULL,
    parent_id      uuid,
    internal_role  text,
    version        bigint NOT NULL,
    schema_version int    NOT NULL,
    -- One Internal Category per role and Household.
    CONSTRAINT categories_internal_role_key UNIQUE (household_id, internal_role)
);

CREATE INDEX categories_household_idx ON categories (household_id);

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['households', 'household_members', 'categories'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format($p$
            CREATE POLICY household_isolation ON %I
                USING (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
                WITH CHECK (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
        $p$, t);
    END LOOP;
END
$$;

GRANT SELECT, INSERT, UPDATE ON households, categories TO wa_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON household_members TO wa_app;

-- Lookup row of the Household fold, in instance scope: which Households a
-- User belongs to, so a User's Households can be listed before any Household
-- is set. It holds IDs only, never Household data (ADR-0014, ADR-0017).
CREATE TABLE user_households (
    user_id      uuid NOT NULL,
    household_id uuid NOT NULL,
    PRIMARY KEY (user_id, household_id)
);

ALTER TABLE user_households ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_households FORCE ROW LEVEL SECURITY;

CREATE POLICY instance_scope_only ON user_households
    USING (NULLIF(current_setting('app.household_id', true), '') IS NULL)
    WITH CHECK (NULLIF(current_setting('app.household_id', true), '') IS NULL);

GRANT SELECT, INSERT, DELETE ON user_households TO wa_app;
