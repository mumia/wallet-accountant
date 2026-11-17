package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// ErrNotificationNotFound is returned for a Notification that is not one
// of the Household's.
var ErrNotificationNotFound = errors.New("notification not found")

// Notification is a Notification as one member sees it.
type Notification struct {
	ID                  uuid.UUID
	Kind                domain.NotificationKind
	StatementImportID   uuid.UUID
	ConnectionID        uuid.UUID
	AccountID           uuid.UUID
	SuspectedDuplicates int
	RaisedAt            time.Time
	// Read is whether the member has marked it read.
	Read bool
}

// NotificationReader reads the Notification folds.
type NotificationReader interface {
	// NotificationsFor lists the Notifications of the Household the
	// transaction is set to, newest first, each read or not by user.
	NotificationsFor(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) ([]Notification, error)
}

// Notifications lists a Household's Notifications and marks them read.
type Notifications struct {
	households    *Households
	notifications NotificationAggregate
	reader        NotificationReader
}

// NewNotifications returns the Notifications service.
func NewNotifications(households *Households, notifications NotificationAggregate, reader NotificationReader) *Notifications {
	return &Notifications{households: households, notifications: notifications, reader: reader}
}

// List returns the Household's Notifications, newest first, with the
// caller's read state.
func (n *Notifications) List(ctx context.Context, household uuid.UUID) ([]Notification, error) {
	var list []Notification

	err := n.households.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) error {
		var err error
		list, err = n.reader.NotificationsFor(ctx, tx, user)

		return err //nolint:wrapcheck // the adapter's error
	})
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	return list, nil
}

// MarkRead marks the Notification read for the caller only.
func (n *Notifications) MarkRead(ctx context.Context, household, notification uuid.UUID) (Notification, error) {
	var read Notification

	err := n.households.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) error {
		res, err := n.notifications.Execute(ctx, tx, notification, domain.MarkNotificationRead{UserID: user})
		if errors.Is(err, domain.ErrNotificationNotRaised) {
			return ErrNotificationNotFound
		}

		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the runtime
		}

		read = notificationFor(notification, res.State, user)

		return nil
	})
	if err != nil {
		return Notification{}, fmt.Errorf("mark notification %s read: %w", notification, err)
	}

	return read, nil
}

func notificationFor(id uuid.UUID, s domain.NotificationState, user uuid.UUID) Notification {
	return Notification{
		ID:                  id,
		Kind:                s.Kind,
		StatementImportID:   s.StatementImportID,
		ConnectionID:        s.ConnectionID,
		AccountID:           s.AccountID,
		SuspectedDuplicates: s.SuspectedDuplicates,
		RaisedAt:            s.RaisedAt,
		Read:                s.IsReadBy(user),
	}
}

// SuspectedDuplicatesFound is a Statement import that brought Suspected
// duplicates, as the Household is told about it.
type SuspectedDuplicatesFound struct {
	StatementImportID   uuid.UUID
	ConnectionID        uuid.UUID
	AccountID           uuid.UUID
	SuspectedDuplicates int
}

// NotificationRaiser raises Notifications from deferred event handlers
// (ADR-0013).
type NotificationRaiser struct {
	notifications NotificationAggregate
}

// NewNotificationRaiser returns the NotificationRaiser.
func NewNotificationRaiser(notifications NotificationAggregate) NotificationRaiser {
	return NotificationRaiser{notifications: notifications}
}

// RaiseSuspectedDuplicatesFound raises a "suspected duplicates found"
// Notification linking to the Statement import, in tx, which is set to the
// import's Household.
func (r NotificationRaiser) RaiseSuspectedDuplicatesFound(ctx context.Context, tx *eventstore.Tx, found SuspectedDuplicatesFound) error {
	id, err := sharedkernel.NewID()
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	_, err = r.notifications.Execute(ctx, tx, id, domain.RaiseSuspectedDuplicatesFound{
		StatementImportID:   found.StatementImportID,
		ConnectionID:        found.ConnectionID,
		AccountID:           found.AccountID,
		SuspectedDuplicates: found.SuspectedDuplicates,
	})
	if err != nil {
		return fmt.Errorf("raise suspected duplicates found for statement import %s: %w", found.StatementImportID, err)
	}

	return nil
}
