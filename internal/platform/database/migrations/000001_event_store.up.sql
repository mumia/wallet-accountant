-- The platform's tables: the event store (ADR-0011, ADR-0015), Watermill's
-- offsets table and the processed-events table, plus the roles every
-- connection uses (ADR-0014). Runs as wa_owner, which owns every table.

-- Roles. wa_owner runs this migration, so it exists already: the database
-- bootstrap creates it (deploy/dev/postgres-init.sql in development) with
-- CREATEROLE and BYPASSRLS, which creating wa_event_store_reader requires.
-- Roles are cluster-wide, so a second database in the same cluster reuses them.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'wa_app') THEN
        CREATE ROLE wa_app LOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'wa_event_store_reader') THEN
        CREATE ROLE wa_event_store_reader LOGIN BYPASSRLS;
    END IF;
END
$$;

-- The reader may switch to wa_app inside a handler, but has none of its
-- privileges otherwise.
GRANT wa_app TO wa_event_store_reader WITH INHERIT FALSE, SET TRUE;

GRANT USAGE ON SCHEMA public TO wa_app, wa_event_store_reader;

-- The event store. Attributes are CloudEvents attributes (ADR-0015), named as
-- CloudEvents names them. householdid is NULL for instance-level events.
CREATE TABLE event_store (
    global_position BIGSERIAL   NOT NULL,
    transaction_id  xid8        NOT NULL DEFAULT pg_current_xact_id(),
    id              uuid        NOT NULL UNIQUE,
    source          text        NOT NULL,
    type            text        NOT NULL,
    subject         uuid        NOT NULL,
    time            timestamptz NOT NULL,
    datacontenttype text        NOT NULL,
    dataschema      text,
    householdid     uuid,
    streamtype      text        NOT NULL,
    streamversion   bigint      NOT NULL CHECK (streamversion > 0),
    actorid         uuid,
    correlationid   uuid,
    causationid     uuid,
    traceparent     text,
    tracestate      text,
    data            jsonb       NOT NULL,
    PRIMARY KEY (transaction_id, global_position),
    CONSTRAINT event_store_stream_version_key UNIQUE (subject, streamversion)
);

CREATE INDEX event_store_streamtype_idx ON event_store (streamtype, subject);

ALTER TABLE event_store ENABLE ROW LEVEL SECURITY;
ALTER TABLE event_store FORCE ROW LEVEL SECURITY;

-- A Household's events only while that Household is set; instance-level
-- events (no Household) only while none is set (ADR-0014, ADR-0017).
CREATE POLICY household_or_instance ON event_store
    USING (
        (householdid IS NULL AND NULLIF(current_setting('app.household_id', true), '') IS NULL)
        OR householdid = NULLIF(current_setting('app.household_id', true), '')::uuid
    )
    WITH CHECK (
        (householdid IS NULL AND NULLIF(current_setting('app.household_id', true), '') IS NULL)
        OR householdid = NULLIF(current_setting('app.household_id', true), '')::uuid
    );

-- Append-only: no UPDATE or DELETE for anyone but the owner.
GRANT SELECT, INSERT ON event_store TO wa_app;
GRANT USAGE ON SEQUENCE event_store_global_position_seq TO wa_app;
GRANT SELECT ON event_store TO wa_event_store_reader;

-- Watermill's offsets table, in the shape of its DefaultPostgreSQLOffsetsAdapter.
CREATE TABLE watermill_offsets_events (
    consumer_group                VARCHAR(255) NOT NULL PRIMARY KEY,
    offset_acked                  BIGINT,
    last_processed_transaction_id xid8         NOT NULL
);

GRANT SELECT, INSERT, UPDATE ON watermill_offsets_events TO wa_event_store_reader;

-- Events each consumer group has handled, written in the handler's
-- transaction so a redelivery changes nothing (ADR-0011). It holds no
-- Household data, only event IDs.
CREATE TABLE processed_events (
    consumer_group text NOT NULL,
    event_id       uuid NOT NULL,
    PRIMARY KEY (consumer_group, event_id)
);

GRANT SELECT, INSERT ON processed_events TO wa_app;
