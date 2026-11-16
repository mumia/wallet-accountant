// Package postgres is the Ledger PostgreSQL adapter: the fold stores of the
// Ledger aggregates and the reads of their folds. Every table is
// Household-scoped, and every query also filters by the transaction's
// Household (ADR-0014).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/ledger/app"
	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// CategoryFolds stores Category folds.
type CategoryFolds struct{}

var _ eventstore.FoldStore[domain.CategoryState] = CategoryFolds{}

// Load implements eventstore.FoldStore.
func (CategoryFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.CategoryState], bool, error) {
	var (
		f      eventstore.Fold[domain.CategoryState]
		parent *uuid.UUID
		role   *string
	)

	err := tx.QueryRow(ctx, `SELECT name, kind, parent_id, internal_role, version, schema_version
		FROM categories WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id).
		Scan(&f.State.Name, &f.State.Kind, &parent, &role, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.CategoryState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.CategoryState]{}, false, fmt.Errorf("load category: %w", err)
	}

	f.State.Created = true

	if parent != nil {
		f.State.ParentID = *parent
	}

	if role != nil {
		f.State.InternalRole = domain.InternalRole(*role)
	}

	return f, true, nil
}

// Save implements eventstore.FoldStore.
func (CategoryFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.CategoryState], expected int64) error {
	s := f.State

	var parent *uuid.UUID
	if s.ParentID != uuid.Nil {
		parent = &s.ParentID
	}

	var role *string
	if s.InternalRole != "" {
		role = (*string)(&s.InternalRole)
	}

	args := []any{tx.HouseholdID(), id, s.Name, s.Kind, parent, role, f.Version, f.SchemaVersion}
	q := `INSERT INTO categories (household_id, id, name, kind, parent_id, internal_role, version, schema_version)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE categories SET name = $3, kind = $4, parent_id = $5, internal_role = $6, version = $7, schema_version = $8
		     WHERE household_id = $1 AND id = $2 AND version = $9`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save category: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	return nil
}

// CategoryReader implements app.CategoryReader.
type CategoryReader struct{}

var _ app.CategoryReader = CategoryReader{}

// Categories implements app.CategoryReader. Category IDs are UUIDv7, so the
// Categories come in the order they were created.
func (CategoryReader) Categories(ctx context.Context, tx *eventstore.Tx) ([]app.Category, error) {
	rows, err := tx.Query(ctx, `SELECT id, name, kind, COALESCE(parent_id, '00000000-0000-0000-0000-000000000000')
		FROM categories WHERE household_id = $1 ORDER BY id`, tx.HouseholdID())
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.Category, error) {
		var c app.Category
		err := r.Scan(&c.ID, &c.Name, &c.Kind, &c.ParentID)

		return c, err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	return list, nil
}
