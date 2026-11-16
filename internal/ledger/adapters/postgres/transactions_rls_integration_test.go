//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
)

// Row-level security alone, with no query filter, isolates the tables of
// the Transaction fold and the Account's running balance (ADR-0014).

func TestAccountMonthsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "account_months",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO account_months (household_id, account_id, booking_month, net_change)
				VALUES ($1, $2, '2026-01', 10.50)`, household, uuid.New())

			return err
		},
	})
}

func TestLedgerTransactionsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "ledger_transactions",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO ledger_transactions (household_id, id, opening_balance, booking_date,
				transaction_date, description, payee, postings, version, schema_version)
				VALUES ($1, $2, true, '2026-01-01', '2026-01-01', 'Opening balance', '', '[]', 1, 1)`, household, uuid.New())

			return err
		},
	})
}

func TestAccountPostingsAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "account_postings",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO account_postings (household_id, transaction_id, ordinal, account_id,
				booking_date, transaction_date, description, amount, currency, category_id)
				VALUES ($1, $2, 0, $3, '2026-01-01', '2026-01-01', 'Opening balance', 10.50, 'EUR', $4)`,
				household, uuid.New(), uuid.New(), uuid.New())

			return err
		},
	})
}
