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

// AccountFolds stores Account folds.
type AccountFolds struct{}

var _ eventstore.FoldStore[domain.AccountState] = AccountFolds{}

// Load implements eventstore.FoldStore.
func (AccountFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.AccountState], bool, error) {
	var f eventstore.Fold[domain.AccountState]

	err := tx.QueryRow(ctx, `SELECT name, kind, currency, start_date, awaiting_first_statement_import, version, schema_version
		FROM accounts WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id).
		Scan(&f.State.Name, &f.State.Kind, &f.State.Currency, &f.State.StartDate,
			&f.State.AwaitingFirstStatementImport, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.AccountState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.AccountState]{}, false, fmt.Errorf("load account: %w", err)
	}

	f.State.Added = true

	return f, true, nil
}

// Save implements eventstore.FoldStore.
func (AccountFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.AccountState], expected int64) error {
	s := f.State
	args := []any{
		tx.HouseholdID(), id, s.Name, s.Kind, s.Currency, s.StartDate, s.AwaitingFirstStatementImport,
		f.Version, f.SchemaVersion,
	}
	q := `INSERT INTO accounts (household_id, id, name, kind, currency, start_date, awaiting_first_statement_import,
	          version, schema_version)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE accounts SET name = $3, kind = $4, currency = $5, start_date = $6,
		         awaiting_first_statement_import = $7, version = $8, schema_version = $9
		     WHERE household_id = $1 AND id = $2 AND version = $10`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save account: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	return nil
}

// AccountReader implements app.AccountReader.
type AccountReader struct{}

var _ app.AccountReader = AccountReader{}

// Accounts implements app.AccountReader. Account IDs are UUIDv7, so the
// Accounts come in the order they were added.
func (AccountReader) Accounts(ctx context.Context, tx *eventstore.Tx) ([]app.Account, error) {
	rows, err := tx.Query(ctx, `SELECT id, name, kind, currency, start_date, awaiting_first_statement_import
		FROM accounts WHERE household_id = $1 ORDER BY id`, tx.HouseholdID())
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.Account, error) {
		var a app.Account
		err := r.Scan(&a.ID, &a.Name, &a.Kind, &a.Currency, &a.StartDate, &a.AwaitingFirstStatementImport)

		return a, err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	return list, nil
}
