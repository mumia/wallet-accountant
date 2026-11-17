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

func TestConnectionsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "connections",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO connections (household_id, id, account_id, kind, csv_mapping,
				csv_mapping_version, version, schema_version)
				VALUES ($1, $2, $3, 'statement_file_import', '{"delimiter": ";"}', 1, 2, 1)`,
				household, uuid.New(), uuid.New())

			return err
		},
	})
}
