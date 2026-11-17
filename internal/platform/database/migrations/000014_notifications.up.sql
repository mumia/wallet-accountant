-- Households: the Notification fold, a Household-level message every member
-- sees, with read state per User (spec "Households: Notification"). Both
-- tables are Household-scoped: they have household_id, forced row-level
-- security and the standard policy (ADR-0014).

CREATE TABLE notifications (
    household_id         uuid        NOT NULL,
    notification_id      uuid        PRIMARY KEY,
    kind                 text        NOT NULL,
    statement_import_id  uuid        NOT NULL,
    connection_id        uuid        NOT NULL,
    account_id           uuid        NOT NULL,
    suspected_duplicates int         NOT NULL,
    raised_at            timestamptz NOT NULL,
    version              bigint      NOT NULL,
    schema_version       int         NOT NULL
);

-- Notification IDs are UUIDv7, so the newest come first in this order.
CREATE INDEX notifications_household_idx ON notifications (household_id, notification_id DESC);

-- Lookup row of the Notification fold: one per User who marked it read.
CREATE TABLE notification_reads (
    household_id    uuid NOT NULL,
    notification_id uuid NOT NULL REFERENCES notifications (notification_id),
    user_id         uuid NOT NULL,
    PRIMARY KEY (notification_id, user_id)
);

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['notifications', 'notification_reads'] LOOP
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

GRANT SELECT, INSERT, UPDATE ON notifications TO wa_app;
GRANT SELECT, INSERT, DELETE ON notification_reads TO wa_app;
