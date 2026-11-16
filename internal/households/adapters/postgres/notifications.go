package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// NotificationFolds stores Notification folds with their lookup rows: one
// per User who marked the Notification read. The transaction must be set to
// the Notification's Household.
type NotificationFolds struct{}

var _ eventstore.FoldStore[domain.NotificationState] = NotificationFolds{}

// Load implements eventstore.FoldStore.
func (NotificationFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.NotificationState], bool, error) {
	var f eventstore.Fold[domain.NotificationState]

	s := &f.State

	err := tx.QueryRow(ctx, `SELECT kind, statement_import_id, connection_id, account_id, suspected_duplicates, raised_at,
		       version, schema_version
		FROM notifications WHERE household_id = $1 AND notification_id = $2`, tx.HouseholdID(), id).
		Scan(&s.Kind, &s.StatementImportID, &s.ConnectionID, &s.AccountID, &s.SuspectedDuplicates, &s.RaisedAt,
			&f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.NotificationState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.NotificationState]{}, false, fmt.Errorf("load notification: %w", err)
	}

	s.Raised = true

	rows, err := tx.Query(ctx, `SELECT user_id FROM notification_reads
		WHERE household_id = $1 AND notification_id = $2 ORDER BY user_id`, tx.HouseholdID(), id)
	if err != nil {
		return eventstore.Fold[domain.NotificationState]{}, false, fmt.Errorf("load notification reads: %w", err)
	}

	s.ReadBy, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return eventstore.Fold[domain.NotificationState]{}, false, fmt.Errorf("load notification reads: %w", err)
	}

	return f, true, nil
}

// Save implements eventstore.FoldStore.
func (NotificationFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.NotificationState], expected int64) error {
	s := f.State
	household := tx.HouseholdID()

	args := []any{
		household, id, string(s.Kind), s.StatementImportID, s.ConnectionID, s.AccountID, s.SuspectedDuplicates,
		s.RaisedAt, f.Version, f.SchemaVersion,
	}
	q := `INSERT INTO notifications (household_id, notification_id, kind, statement_import_id, connection_id, account_id,
	          suspected_duplicates, raised_at, version, schema_version)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (notification_id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE notifications SET kind = $3, statement_import_id = $4, connection_id = $5, account_id = $6,
		         suspected_duplicates = $7, raised_at = $8, version = $9, schema_version = $10
		     WHERE household_id = $1 AND notification_id = $2 AND version = $11`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save notification: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	readBy := s.ReadBy
	if readBy == nil {
		readBy = []uuid.UUID{}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM notification_reads
		WHERE household_id = $1 AND notification_id = $2 AND NOT user_id = ANY($3)`, household, id, readBy); err != nil {
		return fmt.Errorf("save notification reads: %w", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO notification_reads (household_id, notification_id, user_id)
		SELECT $1, $2, unnest($3::uuid[]) ON CONFLICT DO NOTHING`, household, id, readBy); err != nil {
		return fmt.Errorf("save notification reads: %w", err)
	}

	return nil
}

// NotificationReader implements app.NotificationReader.
type NotificationReader struct{}

var _ app.NotificationReader = NotificationReader{}

// NotificationsFor implements app.NotificationReader. Notification IDs are
// UUIDv7, so ordering by them is ordering by when they were raised.
func (NotificationReader) NotificationsFor(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) ([]app.Notification, error) {
	rows, err := tx.Query(ctx, `SELECT n.notification_id, n.kind, n.statement_import_id, n.connection_id, n.account_id,
		       n.suspected_duplicates, n.raised_at,
		       EXISTS (SELECT 1 FROM notification_reads r
		               WHERE r.household_id = n.household_id AND r.notification_id = n.notification_id AND r.user_id = $2)
		FROM notifications n WHERE n.household_id = $1 ORDER BY n.notification_id DESC`, tx.HouseholdID(), user)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.Notification, error) {
		var n app.Notification

		err := r.Scan(&n.ID, &n.Kind, &n.StatementImportID, &n.ConnectionID, &n.AccountID, &n.SuspectedDuplicates,
			&n.RaisedAt, &n.Read)

		return n, err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	return list, nil
}
