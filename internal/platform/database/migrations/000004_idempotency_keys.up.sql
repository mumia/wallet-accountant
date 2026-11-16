-- Idempotency-Key for every POST endpoint (conventions.md, Zalando). A key
-- is reserved in instance scope before the request runs, so a concurrent
-- retry can't run it twice. A successful response is stored where its data
-- belongs: a response holding a Household's data in that Household's scope,
-- any other response with the key itself (ADR-0014).

-- Instance scope. key_id is the SHA-256 of the caller's OIDC issuer and
-- subject and the key, so neither the identity nor the key is stored, and
-- one caller's keys never collide with another's. It holds IDs and hashes,
-- never Household data.
CREATE TABLE idempotency_keys (
    key_id       text        PRIMARY KEY,
    request_hash text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    -- The Household the stored response belongs to, if any.
    household_id uuid,
    -- The stored response, when it belongs to no Household.
    status       int,
    content_type text,
    location     text,
    body         bytea
);

ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_keys FORCE ROW LEVEL SECURITY;

CREATE POLICY instance_scope_only ON idempotency_keys
    USING (NULLIF(current_setting('app.household_id', true), '') IS NULL)
    WITH CHECK (NULLIF(current_setting('app.household_id', true), '') IS NULL);

GRANT SELECT, INSERT, UPDATE, DELETE ON idempotency_keys TO wa_app;

-- Household-scoped: stored responses holding a Household's data.
CREATE TABLE idempotency_responses (
    household_id uuid  NOT NULL,
    key_id       text  NOT NULL,
    status       int   NOT NULL,
    content_type text,
    location     text,
    body         bytea NOT NULL,
    PRIMARY KEY (household_id, key_id)
);

ALTER TABLE idempotency_responses ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_responses FORCE ROW LEVEL SECURITY;

CREATE POLICY household_isolation ON idempotency_responses
    USING (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = NULLIF(current_setting('app.household_id', true), '')::uuid);

GRANT SELECT, INSERT ON idempotency_responses TO wa_app;
