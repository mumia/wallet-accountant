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

// foldRoundTrip saves each state in turn as the next version of the fold
// id, and checks that it loads back exactly as saved.
func foldRoundTrip[S any](t *testing.T, folds eventstore.FoldStore[S], id uuid.UUID, states ...S) {
	t.Helper()

	urls := databasetest.New(t)

	pool, err := pgxpool.New(t.Context(), urls.App)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)

	store := eventstore.NewStore(pool)
	scope := eventstore.Scope{HouseholdID: uuid.Must(uuid.NewV7())}

	for version, state := range states {
		err := store.InTx(t.Context(), scope, func(ctx context.Context, tx *eventstore.Tx) error {
			return folds.Save(ctx, tx, id, eventstore.Fold[S]{State: state, Version: int64(version + 1), SchemaVersion: 1},
				int64(version))
		})
		if err != nil {
			t.Fatalf("save version %d: %v", version+1, err)
		}

		var (
			got   eventstore.Fold[S]
			found bool
		)

		err = store.InTx(t.Context(), scope, func(ctx context.Context, tx *eventstore.Tx) error {
			var err error

			got, found, err = folds.Load(ctx, tx, id)

			return err
		})
		if err != nil || !found {
			t.Fatalf("load version %d: found %v, %v", version+1, found, err)
		}

		if !reflect.DeepEqual(got.State, state) {
			t.Errorf("version %d loaded as %+v, want %+v", version+1, got.State, state)
		}
	}
}

// A Match loads back as confirmed, then as removed: getMatch and the leg
// Reverse read it.
func TestAMatchFoldLoadsAsSaved(t *testing.T) {
	t.Parallel()

	confirmed := domain.MatchState{
		Confirmed: true, Kind: domain.MatchKindAccountTransfer,
		Legs: [2]domain.MatchLeg{
			{TransactionID: uuid.Must(uuid.NewV7()), AccountID: uuid.Must(uuid.NewV7())},
			{TransactionID: uuid.Must(uuid.NewV7()), AccountID: uuid.Must(uuid.NewV7())},
		},
		ConfirmedBy: uuid.Must(uuid.NewV7()),
		ConfirmedAt: time.Date(2026, time.November, 25, 18, 0, 0, 123000, time.UTC),
	}

	removed := confirmed
	removed.Removed = true

	foldRoundTrip(t, eventstore.FoldStore[domain.MatchState](postgres.MatchFolds{}), uuid.Must(uuid.NewV7()),
		confirmed, removed)
}

// A Transfer leg loads back with its counterpart Account and automatic flag,
// In transit, then Matched, then unmatched again.
func TestATransferLegFoldLoadsAsSaved(t *testing.T) {
	t.Parallel()

	wallet, main := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	amount, err := sharedkernel.ParseDecimal("50")
	if err != nil {
		t.Fatal(err)
	}

	inTransit := domain.TransactionState{
		Recorded:             true,
		BookingDate:          time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC),
		TransactionDate:      time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC),
		Description:          "ATM",
		CounterpartAccountID: main,
		CreatedByTransfer:    true,
		Postings: []domain.Posting{
			{AccountID: wallet, Amount: amount, Currency: "EUR"},
			{CategoryID: uuid.Must(uuid.NewV7()), Amount: amount.Neg(), Currency: "EUR"},
		},
	}

	matched := inTransit
	matched.MatchID = uuid.Must(uuid.NewV7())

	foldRoundTrip(t, eventstore.FoldStore[domain.TransactionState](postgres.TransactionFolds{}), uuid.Must(uuid.NewV7()),
		inTransit, matched, inTransit)
}
