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

// notificationColumns are the columns notificationRow scans.
const notificationColumns = `n.kind, n.statement_import_id, n.connection_id, n.account_id, n.suspected_duplicates,
	n.member_id, n.removed_by, n.raised_at`

// notificationRow scans a notifications row, whose columns of other kinds
// are NULL.
type notificationRow struct {
	kind                                         domain.NotificationKind
	statementImport, connection, account, member *uuid.UUID
	removedBy                                    *uuid.UUID
	suspectedDuplicates                          *int
	raisedAt                                     time.Time
}

func (r *notificationRow) targets() []any {
	return []any{
		&r.kind, &r.statementImport, &r.connection, &r.account, &r.suspectedDuplicates, &r.member, &r.removedBy,
		&r.raisedAt,
	}
}

func (r *notificationRow) state() domain.NotificationState {
	return domain.NotificationState{
		Raised:              true,
		Kind:                r.kind,
		StatementImportID:   derefID(r.statementImport),
		ConnectionID:        derefID(r.connection),
		AccountID:           derefID(r.account),
		SuspectedDuplicates: derefInt(r.suspectedDuplicates),
		MemberID:            derefID(r.member),
		RemovedBy:           derefID(r.removedBy),
		RaisedAt:            r.raisedAt,
	}
}

// NotificationFolds stores Notification folds with their lookup rows: one
// per User who marked the Notification read. The transaction must be set to
// the Notification's Household.
type NotificationFolds struct{}

var _ eventstore.FoldStore[domain.NotificationState] = NotificationFolds{}

// Load implements eventstore.FoldStore.
func (NotificationFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.NotificationState], bool, error) {
	var (
		f   eventstore.Fold[domain.NotificationState]
		row notificationRow
	)

	err := tx.QueryRow(ctx, `SELECT `+notificationColumns+`, n.version, n.schema_version
		FROM notifications n WHERE n.household_id = $1 AND n.notification_id = $2`, tx.HouseholdID(), id).
		Scan(append(row.targets(), &f.Version, &f.SchemaVersion)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.NotificationState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.NotificationState]{}, false, fmt.Errorf("load notification: %w", err)
	}

	f.State = row.state()

	rows, err := tx.Query(ctx, `SELECT user_id FROM notification_reads
		WHERE household_id = $1 AND notification_id = $2 ORDER BY user_id`, tx.HouseholdID(), id)
	if err != nil {
		return eventstore.Fold[domain.NotificationState]{}, false, fmt.Errorf("load notification reads: %w", err)
	}

	f.State.ReadBy, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
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
		household, id, string(s.Kind), nullID(s.StatementImportID), nullID(s.ConnectionID), nullID(s.AccountID),
		nullInt(s.SuspectedDuplicates), nullID(s.MemberID), nullID(s.RemovedBy), s.RaisedAt, f.Version, f.SchemaVersion,
	}
	q := `INSERT INTO notifications (household_id, notification_id, kind, statement_import_id, connection_id, account_id,
	          suspected_duplicates, member_id, removed_by, raised_at, version, schema_version)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT (notification_id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE notifications SET kind = $3, statement_import_id = $4, connection_id = $5, account_id = $6,
		         suspected_duplicates = $7, member_id = $8, removed_by = $9, raised_at = $10, version = $11,
		         schema_version = $12
		     WHERE household_id = $1 AND notification_id = $2 AND version = $13`
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
	return notifications(ctx, tx, user, `ORDER BY n.notification_id DESC`)
}

// NotificationFor implements app.NotificationReader.
func (NotificationReader) NotificationFor(ctx context.Context, tx *eventstore.Tx, user, id uuid.UUID) (app.Notification, bool, error) {
	list, err := notifications(ctx, tx, user, `AND n.notification_id = $3`, id)
	if err != nil || len(list) == 0 {
		return app.Notification{}, false, err
	}

	return list[0], true, nil
}

// notifications reads the Household's Notifications, each read or not by
// user, filtered and ordered by tail. The display names of the Users they
// name are read in instance scope, as the members listing reads them, so
// removed members keep theirs.
func notifications(ctx context.Context, tx *eventstore.Tx, user uuid.UUID, tail string, args ...any) ([]app.Notification, error) {
	rows, err := tx.Query(ctx, `SELECT n.notification_id, `+notificationColumns+`,
		       EXISTS (SELECT 1 FROM notification_reads r
		               WHERE r.household_id = n.household_id AND r.notification_id = n.notification_id AND r.user_id = $2)
		FROM notifications n WHERE n.household_id = $1 `+tail, append([]any{tx.HouseholdID(), user}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.Notification, error) {
		var (
			id   uuid.UUID
			row  notificationRow
			read bool
		)

		err := r.Scan(append(append([]any{&id}, row.targets()...), &read)...)
		s := row.state()

		return app.Notification{
			ID:                  id,
			Kind:                s.Kind,
			StatementImportID:   s.StatementImportID,
			ConnectionID:        s.ConnectionID,
			AccountID:           s.AccountID,
			SuspectedDuplicates: s.SuspectedDuplicates,
			Member:              app.UserRef{ID: s.MemberID},
			RemovedBy:           app.UserRef{ID: s.RemovedBy},
			RaisedAt:            s.RaisedAt,
			Read:                read,
		}, err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	if err := nameUsers(ctx, tx, list); err != nil {
		return nil, err
	}

	return list, nil
}

// nameUsers fills in the display names of the members and removers the
// Notifications name.
func nameUsers(ctx context.Context, tx *eventstore.Tx, list []app.Notification) error {
	var users []uuid.UUID

	for _, n := range list {
		for _, u := range []uuid.UUID{n.Member.ID, n.RemovedBy.ID} {
			if u != uuid.Nil {
				users = append(users, u)
			}
		}
	}

	if len(users) == 0 {
		return nil
	}

	var names map[uuid.UUID]string

	err := inInstanceScope(ctx, tx, func() error {
		var err error
		names, err = displayNames(ctx, tx, users)

		return err
	})
	if err != nil {
		return err
	}

	for i := range list {
		list[i].Member.DisplayName = names[list[i].Member.ID]
		list[i].RemovedBy.DisplayName = names[list[i].RemovedBy.ID]
	}

	return nil
}

func nullID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}

	return &id
}

func derefID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}

	return *id
}

func nullInt(n int) *int {
	if n == 0 {
		return nil
	}

	return &n
}

func derefInt(n *int) int {
	if n == nil {
		return 0
	}

	return *n
}
