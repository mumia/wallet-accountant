package databasetest

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// These helpers prove that row-level security isolates a table on its own,
// with every query filter bypassed (ADR-0014, spec "Testing Decisions"). They
// work at the PostgreSQL adapter level on purpose: each table a migration
// adds gets one call.

// HouseholdTable describes a Household-scoped table for
// AssertHouseholdIsolation.
type HouseholdTable struct {
	// Name is the table's name.
	Name string
	// Setup, if set, inserts the rows Insert depends on (for example a
	// parent row) in the Household's scope.
	Setup func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error
	// Insert inserts one row of the table for the Household.
	Insert func(ctx context.Context, tx pgx.Tx, household uuid.UUID) error
}

// AssertHouseholdIsolation proves, as wa_app and without filtering any
// query, that a transaction set to one Household neither sees nor writes
// another Household's rows of the table, and that a transaction in instance
// scope neither sees nor writes any of them.
func AssertHouseholdIsolation(t *testing.T, urls URLs, table HouseholdTable) {
	t.Helper()

	ctx := context.Background()
	h1, h2 := uuid.New(), uuid.New()

	for _, h := range []uuid.UUID{h1, h2} {
		InScope(t, urls.App, h, func(tx pgx.Tx) {
			if table.Setup != nil {
				mustSucceed(t, tx, "setup "+table.Name, func() error { return table.Setup(ctx, tx, h) })
			}

			mustSucceed(t, tx, "insert into "+table.Name, func() error { return table.Insert(ctx, tx, h) })
		})
	}

	InScope(t, urls.App, h1, func(tx pgx.Tx) {
		if n := count(t, tx, table.Name); n != 1 {
			t.Errorf("%s: a Household sees %d rows unfiltered, want only its own 1", table.Name, n)
		}

		mustBeRefused(t, tx, "inserting another Household's row into "+table.Name,
			func() error { return table.Insert(ctx, tx, h2) })
		mustBeRefused(t, tx, "moving a row of "+table.Name+" to another Household", func() error {
			_, err := tx.Exec(ctx, "UPDATE "+table.Name+" SET household_id = $1", h2)

			return err //nolint:wrapcheck // checked by mustBeRefused
		})

		for _, statement := range []string{"UPDATE " + table.Name + " SET household_id = household_id", "DELETE FROM " + table.Name} {
			mustTouchAtMost(t, tx, statement, 1)
		}
	})

	InScope(t, urls.App, uuid.Nil, func(tx pgx.Tx) {
		if n := count(t, tx, table.Name); n != 0 {
			t.Errorf("%s: instance scope sees %d Household rows, want 0", table.Name, n)
		}

		mustBeRefused(t, tx, "inserting a Household row of "+table.Name+" in instance scope",
			func() error { return table.Insert(ctx, tx, h1) })
	})

	// Both Households' rows are still there.
	for _, h := range []uuid.UUID{h1, h2} {
		InScope(t, urls.App, h, func(tx pgx.Tx) {
			if n := count(t, tx, table.Name); n != 1 {
				t.Errorf("%s: Household %s has %d rows after the attempts, want 1", table.Name, h, n)
			}
		})
	}
}

// AssertInstanceScopeOnly proves, as wa_app and without filtering any
// query, that a transaction set to a Household neither sees nor writes the
// rows of an instance-level table (ADR-0017). insert inserts one row.
func AssertInstanceScopeOnly(t *testing.T, urls URLs, table string, insert func(ctx context.Context, tx pgx.Tx) error) {
	t.Helper()

	ctx := context.Background()

	InScope(t, urls.App, uuid.Nil, func(tx pgx.Tx) {
		mustSucceed(t, tx, "insert into "+table, func() error { return insert(ctx, tx) })
	})

	InScope(t, urls.App, uuid.New(), func(tx pgx.Tx) {
		if n := count(t, tx, table); n != 0 {
			t.Errorf("%s: a Household sees %d instance rows, want 0", table, n)
		}

		mustBeRefused(t, tx, "inserting into "+table+" with a Household set", func() error { return insert(ctx, tx) })
		mustTouchAtMost(t, tx, "DELETE FROM "+table, 0)
	})

	InScope(t, urls.App, uuid.Nil, func(tx pgx.Tx) {
		if n := count(t, tx, table); n != 1 {
			t.Errorf("%s: instance scope sees %d rows after the attempts, want 1", table, n)
		}
	})
}

// InScope runs fn in a committed transaction on url with app.household_id
// set to household (instance scope for uuid.Nil).
func InScope(t *testing.T, url string, household uuid.UUID, fn func(pgx.Tx)) {
	t.Helper()

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	setting := ""
	if household != uuid.Nil {
		setting = household.String()
	}

	err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.household_id', $1, true)", setting); err != nil {
			return err //nolint:wrapcheck // reported below
		}

		fn(tx)

		return nil
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
}

func count(t *testing.T, tx pgx.Tx, table string) int {
	t.Helper()

	var n int
	if err := tx.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}

	return n
}

// mustSucceed runs fn and fails the test if it errs.
func mustSucceed(t *testing.T, _ pgx.Tx, what string, fn func() error) {
	t.Helper()

	if err := fn(); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// mustBeRefused runs fn in a savepoint and fails the test unless row-level
// security (or a missing privilege) refuses it.
func mustBeRefused(t *testing.T, tx pgx.Tx, what string, fn func() error) {
	t.Helper()

	err := inSavepoint(tx, fn)
	if refused := err != nil &&
		(strings.Contains(err.Error(), "row-level security") || strings.Contains(err.Error(), "permission denied")); !refused {
		t.Errorf("%s: err = %v, want a row-level security violation", what, err)
	}
}

// mustTouchAtMost runs an unfiltered statement, then undoes it, and fails
// the test if it affected more than the transaction's own rows (own): row-level
// security must hide everyone else's. A missing privilege is fine.
func mustTouchAtMost(t *testing.T, tx pgx.Tx, statement string, own int64) {
	t.Helper()

	var affected int64

	err := inSavepoint(tx, func() error {
		tag, err := tx.Exec(context.Background(), statement)
		affected = tag.RowsAffected()

		return err //nolint:wrapcheck // checked below
	})

	switch {
	case err != nil && !strings.Contains(err.Error(), "permission denied"):
		t.Errorf("%s: %v", statement, err)
	case affected > own:
		t.Errorf("%s affected %d rows, want at most the transaction's own %d", statement, affected, own)
	}
}

// inSavepoint runs fn and always rolls back to before it, so an expected
// error leaves the transaction usable and an attempt changes nothing.
func inSavepoint(tx pgx.Tx, fn func() error) error {
	ctx := context.Background()

	if _, err := tx.Exec(ctx, "SAVEPOINT attempt"); err != nil {
		return err //nolint:wrapcheck // a test helper
	}

	err := fn()

	if _, rerr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT attempt"); rerr != nil && err == nil {
		return rerr //nolint:wrapcheck // a test helper
	}

	return err
}
