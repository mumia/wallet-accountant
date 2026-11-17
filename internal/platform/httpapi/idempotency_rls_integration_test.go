//go:build integration

package httpapi_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
)

// Row-level security alone, with no query filter, keeps stored responses in
// their Household and Idempotency-Key reservations in instance scope
// (ADR-0014, spec "Testing Decisions").

func TestStoredResponsesAreIsolatedByRowLevelSecurity(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertHouseholdIsolation(t, urls, databasetest.HouseholdTable{
		Name: "idempotency_responses",
		Insert: func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
			_, err := tx.Exec(ctx, `INSERT INTO idempotency_responses (household_id, key_id, status, body)
				VALUES ($1, $2, 201, '{}')`, household, uuid.NewString())

			return err
		},
	})
}

func TestIdempotencyKeysAreOnlyInInstanceScope(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	databasetest.AssertInstanceScopeOnly(t, urls, "idempotency_keys", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO idempotency_keys (key_id, request_hash) VALUES ($1, 'hash')`, uuid.NewString())

		return err
	})
}
