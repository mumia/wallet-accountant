//go:build integration

package postgres_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mumia/wallet-accountant/internal/ledger/adapters/postgres"
	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// An Account fold loads back exactly as it was saved, save after save: a
// month that changed is updated, an unchanged one kept, a new one added,
// and one no map holds any more is gone. Rebuild relies on it.
func TestAnAccountFoldLoadsAsSaved(t *testing.T) {
	t.Parallel()

	urls := databasetest.New(t)

	pool, err := pgxpool.New(t.Context(), urls.App)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)

	store := eventstore.NewStore(pool)
	scope := eventstore.Scope{HouseholdID: uuid.Must(uuid.NewV7())}
	account := uuid.Must(uuid.NewV7())
	closedBy := uuid.Must(uuid.NewV7())
	closedAt := time.Date(2026, time.February, 3, 10, 0, 0, 123000, time.UTC)

	dec := func(s string) sharedkernel.Decimal {
		d, err := sharedkernel.ParseDecimal(s)
		if err != nil {
			t.Fatal(err)
		}

		return d
	}

	first := domain.AccountState{
		Added: true, Name: "Main", Kind: domain.AccountKindBankAccount, Currency: "EUR",
		StartDate:                   time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		OpeningBalanceTransactionID: uuid.Must(uuid.NewV7()),
		Months:                      map[string]sharedkernel.Decimal{"2026-01": dec("87.7"), "2026-02": dec("3000"), "2026-04": dec("-5")},
		Uncategorised:               map[string]int{"2026-02": 1, "2026-04": 2},
		Closed: map[string]domain.MonthClose{
			"2026-01": {StatementBalance: dec("87.7"), ClosedAt: closedAt, ClosedBy: closedBy},
		},
	}

	second := first
	second.Months = map[string]sharedkernel.Decimal{"2026-01": dec("85.7"), "2026-02": dec("3000"), "2026-03": dec("-2")}
	second.Uncategorised = map[string]int{"2026-02": 1}
	second.Closed = map[string]domain.MonthClose{
		"2026-01": {StatementBalance: dec("87.7"), ClosedAt: closedAt, ClosedBy: closedBy, Flagged: true},
		"2026-02": {StatementBalance: dec("3087.7"), ClosedAt: closedAt, ClosedBy: closedBy},
	}

	for version, state := range []domain.AccountState{first, second} {
		err := store.InTx(t.Context(), scope, func(ctx context.Context, tx *eventstore.Tx) error {
			return postgres.AccountFolds{}.Save(ctx, tx, account,
				eventstore.Fold[domain.AccountState]{State: state, Version: int64(version + 1), SchemaVersion: 1},
				int64(version))
		})
		if err != nil {
			t.Fatalf("save version %d: %v", version+1, err)
		}

		var got eventstore.Fold[domain.AccountState]

		err = store.InTx(t.Context(), scope, func(ctx context.Context, tx *eventstore.Tx) error {
			var err error

			got, _, err = postgres.AccountFolds{}.Load(ctx, tx, account)

			return err
		})
		if err != nil {
			t.Fatalf("load version %d: %v", version+1, err)
		}

		if !reflect.DeepEqual(got.State, state) {
			t.Errorf("version %d loaded as %+v, want %+v", version+1, got.State, state)
		}
	}
}
