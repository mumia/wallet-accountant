-- The first Statement import (ADR-0018): Ingestion's Statement import and
-- Imported transaction folds, Ledger's Transaction fold with one lookup row
-- per Account Posting (ADR-0011 "Reads"), and the Account fold's Opening
-- balance and running balance per Booking month. Every table is
-- Household-scoped: household_id, forced row-level security and the
-- standard policy (ADR-0014).

-- Ledger: the Account fold gains its Opening balance Transaction (NULL while
-- it awaits its first Statement import) and a running balance per Booking
-- month, kept as lookup rows of the fold.
ALTER TABLE accounts ADD COLUMN opening_balance_transaction_id uuid;

CREATE TABLE account_months (
    household_id  uuid    NOT NULL,
    account_id    uuid    NOT NULL,
    booking_month text    NOT NULL,
    net_change    numeric NOT NULL,
    PRIMARY KEY (account_id, booking_month)
);

CREATE INDEX account_months_household_idx ON account_months (household_id, account_id);

-- Ledger: the Transaction fold. Its Postings sum to zero per currency.
CREATE TABLE ledger_transactions (
    household_id     uuid    NOT NULL,
    id               uuid    PRIMARY KEY,
    opening_balance  boolean NOT NULL,
    booking_date     date    NOT NULL,
    transaction_date date    NOT NULL,
    value_date       date,
    description      text    NOT NULL,
    payee            text    NOT NULL,
    postings         jsonb   NOT NULL,
    version          bigint  NOT NULL,
    schema_version   int     NOT NULL
);

CREATE INDEX ledger_transactions_household_idx ON ledger_transactions (household_id);

-- Ledger: one lookup row per Account Posting of a Transaction, saved with
-- its fold: what an Account's single-entry list shows (ADR-0001).
CREATE TABLE account_postings (
    household_id     uuid    NOT NULL,
    transaction_id   uuid    NOT NULL,
    ordinal          int     NOT NULL,
    account_id       uuid    NOT NULL,
    booking_date     date    NOT NULL,
    transaction_date date    NOT NULL,
    description      text    NOT NULL,
    amount           numeric NOT NULL,
    currency         text    NOT NULL,
    category_id      uuid,
    PRIMARY KEY (transaction_id, ordinal)
);

CREATE INDEX account_postings_list_idx ON account_postings (household_id, account_id, booking_date, transaction_id, ordinal);

-- Ingestion: the Statement import fold. A file is imported once per
-- Connection, by its hash.
CREATE TABLE statement_imports (
    household_id    uuid        NOT NULL,
    id              uuid        PRIMARY KEY,
    connection_id   uuid        NOT NULL,
    account_id      uuid        NOT NULL,
    file_hash       text        NOT NULL,
    mapping_version int         NOT NULL,
    currency        text        NOT NULL,
    export_balance  numeric     NOT NULL,
    rows_imported   int         NOT NULL,
    rows_ignored    int         NOT NULL,
    rows_total      numeric     NOT NULL,
    opening_balance numeric,
    balance_check   text        NOT NULL,
    recorded_at     timestamptz NOT NULL,
    version         bigint      NOT NULL,
    schema_version  int         NOT NULL
);

CREATE UNIQUE INDEX statement_imports_file_idx ON statement_imports (household_id, connection_id, file_hash);
CREATE INDEX statement_imports_account_idx ON statement_imports (household_id, account_id);

-- Ingestion: the Imported transaction fold, queryable by Account, Booking
-- month and state.
CREATE TABLE imported_transactions (
    household_id        uuid    NOT NULL,
    id                  uuid    PRIMARY KEY,
    statement_import_id uuid    NOT NULL,
    account_id          uuid    NOT NULL,
    state               text    NOT NULL,
    line                int     NOT NULL,
    booking_date        date    NOT NULL,
    transaction_date    date    NOT NULL,
    value_date          date,
    amount              numeric NOT NULL,
    currency            text    NOT NULL,
    description         text    NOT NULL,
    payee               text    NOT NULL,
    bank_transaction_id text    NOT NULL,
    version             bigint  NOT NULL,
    schema_version      int     NOT NULL
);

CREATE INDEX imported_transactions_list_idx ON imported_transactions (household_id, account_id, booking_date, id);

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['account_months', 'ledger_transactions', 'account_postings', 'statement_imports',
                             'imported_transactions']
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format($p$CREATE POLICY household_isolation ON %I
            USING (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
            WITH CHECK (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)$p$, t);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE ON %I TO wa_app', t);
    END LOOP;
END
$$;

-- Lookup rows are replaced whenever their fold is saved.
GRANT DELETE ON account_months, account_postings TO wa_app;
