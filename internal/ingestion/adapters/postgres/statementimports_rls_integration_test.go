//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
)

// Row-level security alone, with no query filter, isolates the Statement
// import and Imported transaction folds (ADR-0014).

func TestStatementImportsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "statement_imports",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO statement_imports (household_id, id, connection_id, account_id, file_hash,
				mapping_version, currency, export_balance, rows_imported, rows_ignored, rows_total, opening_balance,
				balance_check, recorded_at, version, schema_version)
				VALUES ($1, $2, $3, $4, 'abc', 1, 'EUR', 100.00, 2, 0, 20.00, 80.00, 'passed', now(), 1, 1)`,
				household, uuid.New(), uuid.New(), uuid.New())

			return err
		},
	})
}

func TestImportedTransactionsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "imported_transactions",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO imported_transactions (household_id, id, statement_import_id, account_id,
				state, line, booking_date, transaction_date, amount, currency, description, payee, bank_transaction_id,
				version, schema_version)
				VALUES ($1, $2, $3, $4, 'pending', 2, '2026-01-02', '2026-01-02', -1.50, 'EUR', 'Coffee', '', '', 1, 1)`,
				household, uuid.New(), uuid.New(), uuid.New())

			return err
		},
	})
}
