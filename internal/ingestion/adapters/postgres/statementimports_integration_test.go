//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mumia/wallet-accountant/internal/ingestion/adapters/postgres"
	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// An upload takes its Statement import ID before it waits for the
// Account's lock, so of two concurrent uploads the one with the smaller ID
// can be recorded second. The latest import, and the order imports are
// listed in, follow when they were recorded, as the waiting Balance check
// does (ImportedUpTo), never the IDs.
func TestTheLatestImportIsTheLastOneRecorded(t *testing.T) {
	t.Parallel()

	pool, err := pgxpool.New(t.Context(), databasetest.New(t).App)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)

	store := eventstore.NewStore(pool)
	scope := eventstore.Scope{HouseholdID: uuid.Must(uuid.NewV7())}
	account, connection := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	smaller, greater := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	recorded := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)

	inTx := func(fn func(ctx context.Context, tx *eventstore.Tx) error) {
		t.Helper()

		if err := store.InTx(t.Context(), scope, fn); err != nil {
			t.Fatal(err)
		}
	}

	// greater is recorded first, smaller a second later.
	for n, id := range []uuid.UUID{greater, smaller} {
		inTx(func(ctx context.Context, tx *eventstore.Tx) error {
			return postgres.StatementImportFolds{}.Save(ctx, tx, id, eventstore.Fold[domain.StatementImportState]{
				State: domain.StatementImportState{
					Recorded: true, ConnectionID: connection, AccountID: account, FileHash: id.String(),
					MappingVersion: 1, Currency: "EUR", BalanceCheck: domain.BalanceCheckPassed,
					RecordedAt: recorded.Add(time.Duration(n) * time.Second),
				},
				Version: 1, SchemaVersion: 1,
			}, 0)
		})
	}

	inTx(func(ctx context.Context, tx *eventstore.Tx) error {
		latest, found, err := postgres.StatementImportReader{}.LatestImport(ctx, tx, account)
		if err != nil {
			return err
		}

		if !found || latest != smaller {
			t.Errorf("latest = %s (found %t), want %s, recorded last", latest, found, smaller)
		}

		list, err := postgres.StatementImportReader{}.StatementImports(ctx, tx, connection)
		if err != nil {
			return err
		}

		if len(list) != 2 || list[0].ID != greater || list[1].ID != smaller {
			t.Errorf("imports = %+v, want %s then %s, in the order they were recorded", list, greater, smaller)
		}

		return nil
	})
}
