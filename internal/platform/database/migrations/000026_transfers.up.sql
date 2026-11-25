-- Ledger: Transfers (.scratch/cash-accounts-and-transfers, ticket 05). A
-- Transfer leg names its counterpart Account; a cash leg an Accept as
-- Transfer recorded on its own says so; a Matched leg names its Match. The
-- Transaction fold's schema version is bumped, so stored folds rebuild with
-- them.
ALTER TABLE ledger_transactions
    ADD COLUMN counterpart_account_id uuid,
    ADD COLUMN created_by_transfer    boolean NOT NULL DEFAULT false,
    ADD COLUMN match_id               uuid;

-- In transit legs: unmatched, non-reversed Transfer legs.
CREATE INDEX ledger_transactions_in_transit_idx ON ledger_transactions (household_id, id)
    WHERE counterpart_account_id IS NOT NULL AND match_id IS NULL AND reversed_by_transaction_id IS NULL;

-- Ledger: the Match fold. A Match links two Transfer legs; a removed one is
-- kept, flagged, as its stream is.
CREATE TABLE matches (
    household_id          uuid        NOT NULL,
    id                    uuid        PRIMARY KEY,
    kind                  text        NOT NULL,
    first_transaction_id  uuid        NOT NULL,
    first_account_id      uuid        NOT NULL,
    second_transaction_id uuid        NOT NULL,
    second_account_id     uuid        NOT NULL,
    confirmed_by          uuid,
    confirmed_at          timestamptz NOT NULL,
    removed               boolean     NOT NULL DEFAULT false,
    version               bigint      NOT NULL,
    schema_version        int         NOT NULL
);

CREATE INDEX matches_household_idx ON matches (household_id);

ALTER TABLE matches ENABLE ROW LEVEL SECURITY;
ALTER TABLE matches FORCE ROW LEVEL SECURITY;
CREATE POLICY household_isolation ON matches
    USING (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE ON matches TO wa_app;
