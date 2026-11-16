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
