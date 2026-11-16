-- Ledger: the Account fold (ADR-0004, ADR-0011). An Account awaits its first
-- Statement import, which records its Opening balance (ADR-0018); there is
-- no column for a typed Opening balance. Household-scoped: household_id,
-- forced row-level security and the standard policy (ADR-0014).

CREATE TABLE accounts (
    household_id                    uuid    NOT NULL,
    id                              uuid    PRIMARY KEY,
    name                            text    NOT NULL,
    kind                            text    NOT NULL,
    currency                        text    NOT NULL,
    start_date                      date    NOT NULL,
    awaiting_first_statement_import boolean NOT NULL,
    version                         bigint  NOT NULL,
    schema_version                  int     NOT NULL
);

CREATE INDEX accounts_household_idx ON accounts (household_id);

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts FORCE ROW LEVEL SECURITY;

CREATE POLICY household_isolation ON accounts
    USING (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE ON accounts TO wa_app;
