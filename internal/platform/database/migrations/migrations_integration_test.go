//go:build integration

package migrations_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
	"github.com/mumia/wallet-accountant/internal/platform/database/migrations"
)

// These tests sit at the PostgreSQL adapter level on purpose: they prove that
// row-level security on the event store blocks access on its own, without
// any query filter (ADR-0014).

func TestMigrationsAreRepeatable(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)

	// The second run goes through databasetest.Migrate too: it sets the
	// cluster-wide role passwords, which must not race other tests' runs.
	databasetest.Migrate(t, func() error { return migrations.Up(t.Context(), urls.Owner, urls.App, urls.Reader) })
}

func TestEventStoreAdmitsOnlyTheHouseholdThatIsSet(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)
	h1, h2 := uuid.New(), uuid.New()

	inScope(t, urls.App, h1.String(), func(tx pgx.Tx) {
		insertEvent(t, tx, &h1)
	})
	inScope(t, urls.App, h2.String(), func(tx pgx.Tx) {
		insertEvent(t, tx, &h2)
	})

	inScope(t, urls.App, h1.String(), func(tx pgx.Tx) {
		if n := countEvents(t, tx); n != 1 {
			t.Errorf("Household 1 sees %d events, want 1", n)
		}

		err := tryInsertEvent(tx, &h2)
		if err == nil || !strings.Contains(err.Error(), "row-level security") {
			t.Errorf("inserting another Household's event: err = %v, want a row-level security violation", err)
		}
	})

	inScope(t, urls.App, "", func(tx pgx.Tx) {
		if n := countEvents(t, tx); n != 0 {
			t.Errorf("instance scope sees %d Household events, want 0", n)
		}
	})
}

func TestInstanceLevelEventsOnlyInInstanceScope(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)
	h1 := uuid.New()

	inScope(t, urls.App, "", func(tx pgx.Tx) {
		insertEvent(t, tx, nil)

		if n := countEvents(t, tx); n != 1 {
			t.Errorf("instance scope sees %d instance-level events, want 1", n)
		}
	})

	inScope(t, urls.App, h1.String(), func(tx pgx.Tx) {
		if n := countEvents(t, tx); n != 0 {
			t.Errorf("a Household sees %d instance-level events, want 0", n)
		}

		err := tryInsertEvent(tx, nil)
		if err == nil || !strings.Contains(err.Error(), "row-level security") {
			t.Errorf("inserting an instance-level event with a Household set: err = %v, want a row-level security violation", err)
		}
	})
}

func TestReaderSeesEveryEventButWritesNothing(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)
	h1, h2 := uuid.New(), uuid.New()

	inScope(t, urls.App, h1.String(), func(tx pgx.Tx) { insertEvent(t, tx, &h1) })
	inScope(t, urls.App, h2.String(), func(tx pgx.Tx) { insertEvent(t, tx, &h2) })
	inScope(t, urls.App, "", func(tx pgx.Tx) { insertEvent(t, tx, nil) })

	inScope(t, urls.Reader, "", func(tx pgx.Tx) {
		if n := countEvents(t, tx); n != 3 {
			t.Errorf("reader sees %d events, want 3", n)
		}
	})

	inScope(t, urls.Reader, h1.String(), func(tx pgx.Tx) {
		err := tryInsertEvent(tx, &h1)
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("reader inserting an event: err = %v, want permission denied", err)
		}
	})

	inScope(t, urls.App, h1.String(), func(tx pgx.Tx) {
		_, err := tx.Exec(t.Context(), "UPDATE event_store SET type = 'changed'")
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("wa_app updating an event: err = %v, want permission denied", err)
		}
	})
}

func TestUsersAndInvitationsOnlyInInstanceScope(t *testing.T) {
	t.Parallel()
	urls := databasetest.New(t)
	ctx := context.Background()
	invitation, user := uuid.New(), uuid.New()

	writes := map[string]func(pgx.Tx) error{
		"invitations": func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO invitations (id, role, code_hash, expires_at, version, schema_version)
				VALUES ($1, 'user', $2, now(), 1, 1)`, invitation, uuid.NewString())

			return err
		},
		"invitation_codes": func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO invitation_codes (code_hash, invitation_id) VALUES ($1, $2)`,
				uuid.NewString(), invitation)

			return err
		},
		"users": func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO users (id, oidc_issuer, oidc_subject, invitation_id, is_instance_administrator, version, schema_version)
				VALUES ($1, 'https://issuer.example', $2, $3, false, 1, 1)`, user, uuid.NewString(), invitation)

			return err
		},
		"user_identities": func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO user_identities (oidc_issuer, oidc_subject, user_id)
				VALUES ('https://issuer.example', $1, $2)`, uuid.NewString(), user)

			return err
		},
	}
	tables := []string{"invitations", "invitation_codes", "users", "user_identities"}

	inScope(t, urls.App, "", func(tx pgx.Tx) {
		for _, table := range tables {
			if err := writes[table](tx); err != nil {
				t.Fatalf("instance scope writing %s: %v", table, err)
			}
		}

		if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT probe; SAVEPOINT probe"); err != nil {
			t.Fatal(err)
		}
	})

	inScope(t, urls.App, uuid.NewString(), func(tx pgx.Tx) {
		for _, table := range tables {
			var n int
			if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
				t.Fatalf("count %s: %v", table, err)
			}

			if n != 0 {
				t.Errorf("a Household sees %d rows of %s, want 0", n, table)
			}

			if _, err := tx.Exec(ctx, "SAVEPOINT w"); err != nil {
				t.Fatal(err)
			}

			err := writes[table](tx)
			if err == nil || !strings.Contains(err.Error(), "row-level security") {
				t.Errorf("a Household writing %s: err = %v, want a row-level security violation", table, err)
			}

			if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT w"); err != nil {
				t.Fatal(err)
			}
		}
	})

	inScope(t, urls.App, "", func(tx pgx.Tx) {
		for _, table := range tables {
			var n int
			if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
				t.Fatalf("count %s: %v", table, err)
			}

			if n != 1 {
				t.Errorf("instance scope sees %d rows of %s, want 1", n, table)
			}
		}
	})
}

// inScope runs fn in a committed transaction on url with app.household_id
// set to household ("" for instance scope).
func inScope(t *testing.T, url, household string, fn func(pgx.Tx)) {
	t.Helper()

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // after commit

	if _, err := tx.Exec(ctx, "SELECT set_config('app.household_id', $1, true)", household); err != nil {
		t.Fatal(err)
	}

	// A savepoint, so an expected error leaves the transaction usable.
	if _, err := tx.Exec(ctx, "SAVEPOINT probe"); err != nil {
		t.Fatal(err)
	}

	fn(tx)

	_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT probe")
	_ = tx.Commit(ctx)
}

func insertEvent(t *testing.T, tx pgx.Tx, household *uuid.UUID) {
	t.Helper()

	if err := tryInsertEvent(tx, household); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	if _, err := tx.Exec(context.Background(), "RELEASE SAVEPOINT probe; SAVEPOINT probe"); err != nil {
		t.Fatal(err)
	}
}

func tryInsertEvent(tx pgx.Tx, household *uuid.UUID) error {
	ctx := context.Background()
	_, err := tx.Exec(ctx, `
		INSERT INTO event_store (id, source, type, subject, time, datacontenttype, householdid, streamtype, streamversion, data)
		VALUES ($1, '/test', 'io.walletaccountant.test.thing.happened.v1', $2, now(), 'application/json', $3, 'test.thing', 1, '{}')`,
		uuid.New(), uuid.New(), household)

	return err
}

func countEvents(t *testing.T, tx pgx.Tx) int {
	t.Helper()

	var n int
	if err := tx.QueryRow(context.Background(), "SELECT count(*) FROM event_store").Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}

	return n
}
