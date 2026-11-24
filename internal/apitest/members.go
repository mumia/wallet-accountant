package apitest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// SeedMember makes the User a member of the Household who joined at
// joinedAt, by writing the Household fold's lookup rows as wa_app: the
// member row in the Household's scope and the membership index in instance
// scope. Tests use it until a User can join a Household through the API.
func (a *App) SeedMember(t testing.TB, householdID, userID string, joinedAt time.Time) {
	t.Helper()

	a.inScope(t, householdID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO household_members (household_id, user_id, joined_at) VALUES ($1, $2, $3)`,
			householdID, userID, joinedAt)

		return err //nolint:wrapcheck // reported by inScope
	})
	a.inScope(t, "", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO user_households (user_id, household_id) VALUES ($1, $2)`, userID, householdID)

		return err //nolint:wrapcheck // reported by inScope
	})
}

// inScope runs fn in a committed wa_app transaction with app.household_id set
// to household ("" for instance scope).
func (a *App) inScope(t testing.TB, household string, fn func(context.Context, pgx.Tx) error) {
	t.Helper()

	ctx := t.Context()

	err := pgx.BeginFunc(ctx, a.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.household_id', $1, true)", household); err != nil {
			return err //nolint:wrapcheck // reported below
		}

		return fn(ctx, tx)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}
