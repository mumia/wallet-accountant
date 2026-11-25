-- Households: Notifications about members joining, leaving and being
-- removed (.scratch/join-household, ticket 10). They name the member and,
-- for removal, the remover, and link to no Statement import, so the import
-- columns become optional. Display names are not copied here: the list
-- reads them in instance scope.
ALTER TABLE notifications
    ALTER COLUMN statement_import_id DROP NOT NULL,
    ALTER COLUMN connection_id DROP NOT NULL,
    ALTER COLUMN account_id DROP NOT NULL,
    ALTER COLUMN suspected_duplicates DROP NOT NULL,
    ADD COLUMN member_id uuid,
    ADD COLUMN removed_by uuid;
