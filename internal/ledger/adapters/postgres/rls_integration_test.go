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

func TestCategoriesAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "categories",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO categories (household_id, id, name, kind, version, schema_version)
				VALUES ($1, $2, 'Food', 'expense', 1, 1)`, household, uuid.New())

			return err
		},
	})
}

func TestAccountsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "accounts",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO accounts (household_id, id, name, kind, currency, start_date,
				awaiting_first_statement_import, version, schema_version)
				VALUES ($1, $2, 'Main', 'bank_account', 'EUR', '2026-01-01', true, 1, 1)`, household, uuid.New())

			return err
		},
	})
}

func TestMatchesAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "matches",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO matches (household_id, id, kind, first_transaction_id, first_account_id,
				second_transaction_id, second_account_id, confirmed_by, confirmed_at, version, schema_version)
				VALUES ($1, $2, 'account_transfer', $3, $4, $5, $6, $7, now(), 1, 1)`,
				household, uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New())

			return err
		},
	})
}
