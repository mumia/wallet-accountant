//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
)

// Row-level security alone, with no query filter, isolates every table this
// adapter adds (ADR-0014, spec "Testing Decisions").

func insertHousehold(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO households (household_id, name, base_currency, version, schema_version)
		VALUES ($1, 'Home', 'EUR', 1, 1)`, household)

	return err
}

func TestHouseholdsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name:   "households",
		Insert: insertHousehold,
	})
}

func TestHouseholdMembersAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name:  "household_members",
		Setup: insertHousehold,
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO household_members (household_id, user_id, joined_at) VALUES ($1, $2, now())`,
				household, uuid.New())

			return err
		},
	})
}

func TestTheMembershipIndexIsOnlyInInstanceScope(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertInstanceScopeOnly(t, urls, "user_households", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO user_households (user_id, household_id) VALUES ($1, $2)`, uuid.New(), uuid.New())

		return err
	})
}

// insertNotification inserts the Household's Notification, whose ID is the
// Household's, so its read rows can refer to it.
func insertNotification(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO notifications (household_id, notification_id, kind, statement_import_id,
		connection_id, account_id, suspected_duplicates, raised_at, version, schema_version)
		VALUES ($1, $1, 'suspected_duplicates_found', $2, $3, $4, 2, now(), 1, 1)`,
		household, uuid.New(), uuid.New(), uuid.New())

	return err
}

func TestNotificationsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name:   "notifications",
		Insert: insertNotification,
	})
}

func TestNotificationReadsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name:  "notification_reads",
		Setup: insertNotification,
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO notification_reads (household_id, notification_id, user_id)
				VALUES ($1, $1, $2)`, household, uuid.New())

			return err
		},
	})
}

func insertHouseholdInvitation(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO household_invitations (household_id, id, code_hash, issued_by, issued_at,
		expires_at, version, schema_version) VALUES ($1, $2, $3, $4, now(), now() + interval '1 day', 1, 1)`,
		household, uuid.New(), uuid.NewString(), uuid.New())

	return err
}

func TestHouseholdInvitationsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name:   "household_invitations",
		Insert: insertHouseholdInvitation,
	})
}

func TestTheCodeLookupRowOfAHouseholdInvitationIsOnlyInInstanceScope(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertInstanceScopeOnly(t, urls, "invitation_codes", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO invitation_codes (code_hash, invitation_id, household_id) VALUES ($1, $2, $3)`,
			uuid.NewString(), uuid.New(), uuid.New())

		return err
	})
}
