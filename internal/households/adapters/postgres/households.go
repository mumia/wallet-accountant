package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// HouseholdFolds stores Household folds with their lookup rows: one member
// row per User in the Household's scope, and the same membership indexed by
// User in instance scope, so a User's Households can be listed.
type HouseholdFolds struct{}

var _ eventstore.FoldStore[domain.HouseholdState] = HouseholdFolds{}

// Load implements eventstore.FoldStore. The transaction must be set to the
// Household.
func (HouseholdFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.HouseholdState], bool, error) {
	var f eventstore.Fold[domain.HouseholdState]

	err := tx.QueryRow(ctx, `SELECT name, base_currency, version, schema_version
		FROM households WHERE household_id = $1 AND household_id = $2`, tx.HouseholdID(), id).
		Scan(&f.State.Name, &f.State.BaseCurrency, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.HouseholdState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.HouseholdState]{}, false, fmt.Errorf("load household: %w", err)
	}

	f.State.Created = true

	rows, err := tx.Query(ctx, `SELECT user_id, joined_at FROM household_members WHERE household_id = $1 ORDER BY user_id`,
		tx.HouseholdID())
	if err != nil {
		return eventstore.Fold[domain.HouseholdState]{}, false, fmt.Errorf("load household members: %w", err)
	}

	f.State.JoinedAt = map[uuid.UUID]time.Time{}

	var (
		user   uuid.UUID
		joined time.Time
	)

	_, err = pgx.ForEachRow(rows, []any{&user, &joined}, func() error {
		f.State.Members = append(f.State.Members, user)
		f.State.JoinedAt[user] = joined.UTC()

		return nil
	})
	if err != nil {
		return eventstore.Fold[domain.HouseholdState]{}, false, fmt.Errorf("load household members: %w", err)
	}

	return f, true, nil
}

// Save implements eventstore.FoldStore. The transaction must be set to the
// Household; it switches to instance scope only to write the membership
// index, and back.
func (HouseholdFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.HouseholdState], expected int64) error {
	household := tx.HouseholdID()
	if household != id {
		return fmt.Errorf("save household %s: %w", id, errWrongHousehold)
	}

	s := f.State

	args := []any{household, s.Name, s.BaseCurrency, f.Version, f.SchemaVersion}
	q := `INSERT INTO households (household_id, name, base_currency, version, schema_version)
	      VALUES ($1, $2, $3, $4, $5) ON CONFLICT (household_id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE households SET name = $2, base_currency = $3, version = $4, schema_version = $5
		     WHERE household_id = $1 AND version = $6`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save household: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	members := s.Members
	if members == nil {
		members = []uuid.UUID{}
	}

	joined := make([]time.Time, len(members))
	for i, m := range members {
		joined[i] = s.JoinedAt[m]
	}

	if _, err := tx.Exec(ctx, `DELETE FROM household_members WHERE household_id = $1 AND NOT user_id = ANY($2)`,
		household, members); err != nil {
		return fmt.Errorf("save household members: %w", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO household_members (household_id, user_id, joined_at)
		SELECT $1, m.user_id, m.joined_at FROM unnest($2::uuid[], $3::timestamptz[]) AS m (user_id, joined_at)
		ON CONFLICT DO NOTHING`, household, members, joined); err != nil {
		return fmt.Errorf("save household members: %w", err)
	}

	return inInstanceScope(ctx, tx, func() error {
		if _, err := tx.Exec(ctx, `DELETE FROM user_households WHERE household_id = $1 AND NOT user_id = ANY($2)`,
			household, members); err != nil {
			return fmt.Errorf("save membership index: %w", err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO user_households (user_id, household_id)
			SELECT unnest($2::uuid[]), $1 ON CONFLICT DO NOTHING`, household, members); err != nil {
			return fmt.Errorf("save membership index: %w", err)
		}

		return nil
	})
}

var errWrongHousehold = errors.New("transaction is set to another household")

// inInstanceScope runs fn with the transaction in instance scope, then sets
// it back to its Household.
func inInstanceScope(ctx context.Context, tx *eventstore.Tx, fn func() error) error {
	household := tx.HouseholdID()

	if err := tx.SetHousehold(ctx, uuid.Nil); err != nil {
		return err //nolint:wrapcheck // already wrapped by the runtime
	}

	if err := fn(); err != nil {
		return err
	}

	return tx.SetHousehold(ctx, household) //nolint:wrapcheck // already wrapped by the runtime
}

// HouseholdLookups implements app.HouseholdLookups.
type HouseholdLookups struct{}

var _ app.HouseholdLookups = HouseholdLookups{}

// HouseholdsOf implements app.HouseholdLookups. Household IDs are UUIDv7, so
// ordering by them is ordering by creation.
func (HouseholdLookups) HouseholdsOf(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `SELECT household_id FROM user_households WHERE user_id = $1 ORDER BY household_id`, user)
	if err != nil {
		return nil, fmt.Errorf("list households of user: %w", err)
	}

	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("list households of user: %w", err)
	}

	return ids, nil
}

// IsMember implements app.HouseholdLookups.
func (HouseholdLookups) IsMember(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) (bool, error) {
	var member bool

	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM household_members WHERE household_id = $1 AND user_id = $2)`,
		tx.HouseholdID(), user).Scan(&member)
	if err != nil {
		return false, fmt.Errorf("check membership: %w", err)
	}

	return member, nil
}

// Members implements app.HouseholdLookups. It reads the member rows in the
// Household's scope, then the display names in instance scope, so no
// display name is copied into the Household (join-household spec,
// "Households: members").
func (HouseholdLookups) Members(ctx context.Context, tx *eventstore.Tx) ([]app.Member, error) {
	rows, err := tx.Query(ctx, `SELECT user_id, joined_at FROM household_members WHERE household_id = $1
		ORDER BY joined_at, user_id`, tx.HouseholdID())
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	members, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (app.Member, error) {
		var m app.Member
		err := row.Scan(&m.UserID, &m.JoinedAt)

		return m, err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}

	names := map[uuid.UUID]string{}

	err = inInstanceScope(ctx, tx, func() error {
		rows, err := tx.Query(ctx, `SELECT id, display_name FROM users WHERE id = ANY($1) AND display_name IS NOT NULL`, ids)
		if err != nil {
			return fmt.Errorf("read display names: %w", err)
		}

		var (
			id   uuid.UUID
			name string
		)

		_, err = pgx.ForEachRow(rows, []any{&id, &name}, func() error {
			names[id] = name

			return nil
		})
		if err != nil {
			return fmt.Errorf("read display names: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	for i := range members {
		members[i].DisplayName = names[members[i].UserID]
	}

	return members, nil
}
