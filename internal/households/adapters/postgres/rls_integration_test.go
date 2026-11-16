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
			_, err := tx.Exec(ctx, `INSERT INTO household_members (household_id, user_id) VALUES ($1, $2)`,
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
