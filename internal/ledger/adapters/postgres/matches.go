package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// MatchFolds stores Match folds. A removed Match keeps its row, flagged.
type MatchFolds struct{}

var _ eventstore.FoldStore[domain.MatchState] = MatchFolds{}

// Load implements eventstore.FoldStore.
func (MatchFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.MatchState], bool, error) {
	var (
		f  eventstore.Fold[domain.MatchState]
		by *uuid.UUID
	)

	s := &f.State

	err := tx.QueryRow(ctx, `SELECT kind, first_transaction_id, first_account_id, second_transaction_id,
			second_account_id, confirmed_by, confirmed_at, removed, version, schema_version
		FROM matches WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id).
		Scan(&s.Kind, &s.Legs[0].TransactionID, &s.Legs[0].AccountID, &s.Legs[1].TransactionID, &s.Legs[1].AccountID,
			&by, &s.ConfirmedAt, &s.Removed, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.MatchState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.MatchState]{}, false, fmt.Errorf("load match: %w", err)
	}

	s.Confirmed = true
	s.ConfirmedAt = s.ConfirmedAt.UTC()

	if by != nil {
		s.ConfirmedBy = *by
	}

	return f, true, nil
}

// Save implements eventstore.FoldStore.
func (MatchFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.MatchState], expected int64) error {
	s := f.State

	args := []any{
		tx.HouseholdID(), id, s.Kind, s.Legs[0].TransactionID, s.Legs[0].AccountID, s.Legs[1].TransactionID,
		s.Legs[1].AccountID, optionalID(s.ConfirmedBy), s.ConfirmedAt, s.Removed, f.Version, f.SchemaVersion,
	}
	q := `INSERT INTO matches (household_id, id, kind, first_transaction_id, first_account_id, second_transaction_id,
	          second_account_id, confirmed_by, confirmed_at, removed, version, schema_version)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE matches SET kind = $3, first_transaction_id = $4, first_account_id = $5,
		         second_transaction_id = $6, second_account_id = $7, confirmed_by = $8, confirmed_at = $9,
		         removed = $10, version = $11, schema_version = $12
		     WHERE household_id = $1 AND id = $2 AND version = $13`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save match: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	return nil
}
