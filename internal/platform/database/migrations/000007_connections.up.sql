-- Ingestion: the Connection fold (ADR-0011). A Connection links an Account
-- to a source of its transactions and holds the CSV mapping of its statement
-- files, whose version changes on every save. Household-scoped: household_id,
-- forced row-level security and the standard policy (ADR-0014).

CREATE TABLE connections (
    household_id        uuid   NOT NULL,
    id                  uuid   PRIMARY KEY,
    account_id          uuid   NOT NULL,
    kind                text   NOT NULL,
    csv_mapping         jsonb,
    csv_mapping_version int    NOT NULL,
    version             bigint NOT NULL,
    schema_version      int    NOT NULL
);

CREATE INDEX connections_household_account_idx ON connections (household_id, account_id);

ALTER TABLE connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE connections FORCE ROW LEVEL SECURITY;

CREATE POLICY household_isolation ON connections
    USING (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE ON connections TO wa_app;
