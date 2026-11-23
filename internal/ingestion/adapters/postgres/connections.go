// Package postgres is the Ingestion PostgreSQL adapter: the fold stores of
// the Ingestion aggregates and the reads of their folds. Every table is
// Household-scoped, and every query also filters by the transaction's
// Household (ADR-0014).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mumia/wallet-accountant/internal/ingestion/app"
	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// ConnectionFolds stores Connection folds. The CSV mapping is kept as JSON.
// A name another Connection of the Account has, ignoring case, is refused
// with app.ErrConnectionNameTaken.
type ConnectionFolds struct{}

var _ eventstore.FoldStore[domain.ConnectionState] = ConnectionFolds{}

// Load implements eventstore.FoldStore.
func (ConnectionFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.ConnectionState], bool, error) {
	var (
		f       eventstore.Fold[domain.ConnectionState]
		mapping []byte
	)

	err := tx.QueryRow(ctx, `SELECT account_id, name, kind, csv_mapping, csv_mapping_version, version, schema_version
		FROM connections WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id).
		Scan(&f.State.AccountID, &f.State.Name, &f.State.Kind, &mapping, &f.State.MappingVersion, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.ConnectionState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.ConnectionState]{}, false, fmt.Errorf("load connection: %w", err)
	}

	f.State.Created = true

	if f.State.CSVMapping, err = decodeMapping(mapping); err != nil {
		return eventstore.Fold[domain.ConnectionState]{}, false, err
	}

	return f, true, nil
}

// Save implements eventstore.FoldStore.
func (ConnectionFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.ConnectionState], expected int64) error {
	s := f.State

	var mapping []byte

	if s.CSVMapping != nil {
		var err error
		if mapping, err = json.Marshal(s.CSVMapping); err != nil {
			return fmt.Errorf("encode CSV mapping: %w", err)
		}
	}

	args := []any{tx.HouseholdID(), id, s.AccountID, s.Kind, mapping, s.MappingVersion, f.Version, f.SchemaVersion, s.Name}
	q := `INSERT INTO connections (household_id, id, account_id, kind, csv_mapping, csv_mapping_version, version, schema_version, name)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE connections SET account_id = $3, kind = $4, csv_mapping = $5, csv_mapping_version = $6,
		         version = $7, schema_version = $8, name = $9
		     WHERE household_id = $1 AND id = $2 AND version = $10`
	}

	tag, err := tx.Exec(ctx, q, args...)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "connections_name_idx" {
		return fmt.Errorf("%w: %q", app.ErrConnectionNameTaken, s.Name)
	}

	if err != nil {
		return fmt.Errorf("save connection: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	return nil
}

// ConnectionReader implements app.ConnectionReader.
type ConnectionReader struct{}

var _ app.ConnectionReader = ConnectionReader{}

// Connections implements app.ConnectionReader. Connection IDs are UUIDv7, so
// the Connections come in the order they were created.
func (ConnectionReader) Connections(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) ([]app.Connection, error) {
	rows, err := tx.Query(ctx, `SELECT id, account_id, name, kind, csv_mapping, csv_mapping_version
		FROM connections WHERE household_id = $1 AND account_id = $2 ORDER BY id`, tx.HouseholdID(), account)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.Connection, error) {
		var (
			c       app.Connection
			mapping []byte
		)

		if err := r.Scan(&c.ID, &c.AccountID, &c.Name, &c.Kind, &mapping, &c.MappingVersion); err != nil {
			return c, err //nolint:wrapcheck // wrapped below
		}

		m, err := decodeMapping(mapping)
		c.CSVMapping = m

		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}

	return list, nil
}

func decodeMapping(raw []byte) (*domain.CSVMapping, error) {
	if raw == nil {
		return nil, nil //nolint:nilnil // no mapping saved yet
	}

	var m domain.CSVMapping
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decode CSV mapping: %w", err)
	}

	return &m, nil
}
