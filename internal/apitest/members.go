package apitest

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
)

// SeedMember makes the User of token a member of household by writing the
// membership rows directly, as wa_owner, without recording an event, so a
// rebuild drops it. It stands in for joining through an Invitation in tests
// that need a second member and don't test joining itself.
func (a *App) SeedMember(t testing.TB, household, token string) {
	t.Helper()

	var me struct {
		UserID string `json:"user_id"`
	}
	a.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &me)

	conn, err := pgx.Connect(t.Context(), a.ownerURL)
	if err != nil {
		t.Fatalf("connect as owner: %v", err)
	}
	defer conn.Close(context.WithoutCancel(t.Context()))

	// Row-level security is forced on the owner too: household_members is
	// in the Household's scope and user_households in instance scope.
	err = pgx.BeginFunc(t.Context(), conn, func(tx pgx.Tx) error {
		for _, step := range []struct {
			scope string
			sql   string
		}{
			{household, `INSERT INTO household_members (household_id, user_id) VALUES ($1, $2)`},
			{"", `INSERT INTO user_households (household_id, user_id) VALUES ($1, $2)`},
		} {
			if _, err := tx.Exec(t.Context(), "SELECT set_config('app.household_id', $1, true)", step.scope); err != nil {
				return err //nolint:wrapcheck // reported below
			}

			if _, err := tx.Exec(t.Context(), step.sql, household, me.UserID); err != nil {
				return err //nolint:wrapcheck // reported below
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("seed member: %v", err)
	}
}
