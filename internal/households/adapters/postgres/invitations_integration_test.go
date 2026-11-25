//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mumia/wallet-accountant/internal/households/adapters/postgres"
	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// An Invitation to a Household is stored in that Household's scope, and
// only its code lookup row in instance scope (spec "Households: Invitation
// in Household scope").

func newPool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func TestAnInvitationToAHouseholdIsStoredInItsScopeWithItsLookupRowInInstanceScope(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	urls := databasetest.New(t)
	store := eventstore.NewStore(newPool(t, urls.App))
	invitations := app.NewInvitationAggregate(postgres.InvitationFolds{})
	household, other, issuer, id := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	code := domain.NewInvitationCode()

	err := store.InTx(ctx, eventstore.Scope{HouseholdID: household, ActorID: issuer}, func(ctx context.Context, tx *eventstore.Tx) error {
		_, err := invitations.Execute(ctx, tx, id, domain.IssueHouseholdInvitation{CodeHash: domain.HashCode(code), At: time.Now()})

		return err
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	loaded := func(scope uuid.UUID) (domain.InvitationState, bool) {
		var (
			state domain.InvitationState
			found bool
		)

		err := store.InTx(ctx, eventstore.Scope{HouseholdID: scope}, func(ctx context.Context, tx *eventstore.Tx) error {
			f, ok, err := invitations.Folds.Load(ctx, tx, id)
			state, found = f.State, ok

			return err
		})
		if err != nil {
			t.Fatalf("load in %s: %v", scope, err)
		}

		return state, found
	}

	if s, ok := loaded(household); !ok || s.HouseholdID != household || s.IssuedBy != issuer || s.Role != domain.RoleUser {
		t.Errorf("in its Household: %+v (found %t), want it issued by %s to %s", s, ok, issuer, household)
	}

	if _, ok := loaded(other); ok {
		t.Error("another Household loads the Invitation")
	}

	if _, ok := loaded(uuid.Nil); ok {
		t.Error("instance scope loads the Invitation")
	}

	lookup := func(scope uuid.UUID) (app.InvitationCode, bool) {
		var (
			got   app.InvitationCode
			found bool
		)

		err := store.InTx(ctx, eventstore.Scope{HouseholdID: scope}, func(ctx context.Context, tx *eventstore.Tx) error {
			var err error
			got, found, err = postgres.Lookups{}.InvitationByCode(ctx, tx, domain.HashCode(code))

			return err
		})
		if err != nil {
			t.Fatalf("look up in %s: %v", scope, err)
		}

		return got, found
	}

	want := app.InvitationCode{InvitationID: id, HouseholdID: household}

	if got, ok := lookup(uuid.Nil); !ok || got != want {
		t.Errorf("instance scope looks the code up as %+v (found %t), want %+v", got, ok, want)
	}

	if _, ok := lookup(household); ok {
		t.Error("the Household's scope sees the code lookup row")
	}

	databasetest.InScope(t, urls.App, uuid.Nil, func(tx pgx.Tx) {
		// Lose the lookup row, which the rebuild below brings back.
		if _, err := tx.Exec(ctx, `DELETE FROM invitation_codes WHERE invitation_id = $1`, id); err != nil {
			t.Fatal(err)
		}
	})

	if err := eventstore.Rebuild(ctx, newPool(t, urls.Reader), store, invitations.RebuildTarget()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	if got, ok := lookup(uuid.Nil); !ok || got != want {
		t.Errorf("after a rebuild, instance scope looks the code up as %+v (found %t), want %+v", got, ok, want)
	}

	if s, ok := loaded(household); !ok || s.IssuedBy != issuer {
		t.Errorf("after a rebuild, in its Household: %+v (found %t)", s, ok)
	}
}
