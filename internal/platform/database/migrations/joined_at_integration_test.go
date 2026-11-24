//go:build integration

package migrations_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
	"github.com/mumia/wallet-accountant/internal/platform/database/migrations"
)

// The last migration before household_members.joined_at.
const beforeJoinedAt = 22

func TestJoinedAtIsBackfilledFromEachMembersMemberAddedEvent(t *testing.T) {
	t.Parallel()
	urls := databasetest.NewEmpty(t)
	ctx := context.Background()

	databasetest.Migrate(t, func() error { return migrations.UpTo(urls.Owner, beforeJoinedAt) })

	home, flat := uuid.New(), uuid.New()
	alice, bob := uuid.New(), uuid.New()
	homeCreated := time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)
	bobJoined := homeCreated.Add(26 * time.Hour)
	flatCreated := homeCreated.Add(72 * time.Hour)

	owner, err := pgx.Connect(ctx, urls.Owner)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close(ctx)

	for _, m := range []struct {
		household, user uuid.UUID
		version         int64
		at              time.Time
	}{
		{home, alice, 2, homeCreated},
		{home, bob, 3, bobJoined},
		{flat, alice, 2, flatCreated},
	} {
		if _, err := owner.Exec(ctx, `INSERT INTO households (household_id, name, base_currency, version, schema_version)
			VALUES ($1, 'H', 'EUR', 3, 1) ON CONFLICT DO NOTHING`, m.household); err != nil {
			t.Fatal(err)
		}

		if _, err := owner.Exec(ctx, `INSERT INTO household_members (household_id, user_id) VALUES ($1, $2)`,
			m.household, m.user); err != nil {
			t.Fatal(err)
		}

		if _, err := owner.Exec(ctx, `
			INSERT INTO event_store (id, source, type, subject, time, datacontenttype, householdid, streamtype, streamversion, actorid, data)
			VALUES ($1, '/households', 'io.walletaccountant.households.household.memberadded.v1', $2, $3,
			        'application/json', $2, 'households.household', $4, $5, jsonb_build_object('userid', $6::text))`,
			uuid.New(), m.household, m.at, m.version, m.user, m.user.String()); err != nil {
			t.Fatal(err)
		}
	}

	databasetest.Migrate(t, func() error { return migrations.Up(ctx, urls.Owner, urls.App, urls.Reader) })

	for _, want := range []struct {
		household, user uuid.UUID
		at              time.Time
	}{
		{home, alice, homeCreated},
		{home, bob, bobJoined},
		{flat, alice, flatCreated},
	} {
		var got time.Time
		if err := owner.QueryRow(ctx, `SELECT joined_at FROM household_members WHERE household_id = $1 AND user_id = $2`,
			want.household, want.user).Scan(&got); err != nil {
			t.Fatal(err)
		}

		if !got.Equal(want.at) {
			t.Errorf("member %s of %s joined at %v, want %v", want.user, want.household, got, want.at)
		}
	}
}
