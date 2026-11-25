package app

import (
	"context"
	"encoding/json"
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

// Notification is a Notification as one member sees it. Fields of other
// kinds are zero.
type Notification struct {
	ID                  uuid.UUID
	Kind                domain.NotificationKind
	StatementImportID   uuid.UUID
	ConnectionID        uuid.UUID
	AccountID           uuid.UUID
	SuspectedDuplicates int
	// Member is who joined, left or was removed, and RemovedBy who removed
	// them, with their current display names.
	Member    UserRef
	RemovedBy UserRef
	RaisedAt  time.Time
	// Read is whether the member has marked it read.
	Read bool
}

// NotificationReader reads the Notification folds.
type NotificationReader interface {
	// NotificationsFor lists the Notifications of the Household the
	// transaction is set to, newest first, each read or not by user.
	NotificationsFor(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) ([]Notification, error)
	// NotificationFor returns one of them, false when the Household has no
	// such Notification.
	NotificationFor(ctx context.Context, tx *eventstore.Tx, user, id uuid.UUID) (Notification, bool, error)
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
		_, err := n.notifications.Execute(ctx, tx, notification, domain.MarkNotificationRead{UserID: user})
		if errors.Is(err, domain.ErrNotificationNotRaised) {
			return ErrNotificationNotFound
		}

		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the runtime
		}

		// Read back, so the response names its Users as the list does.
		var found bool

		read, found, err = n.reader.NotificationFor(ctx, tx, user, notification)
		if err == nil && !found {
			err = ErrNotificationNotFound
		}

		return err
	})
	if err != nil {
		return Notification{}, fmt.Errorf("mark notification %s read: %w", notification, err)
	}

	return read, nil
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

// MembershipNotificationIn reads a Household event and returns the
// Notification it raises (join-household spec, "Households:
// Notifications"): a member who joined through an Invitation, so not the
// creator of a new Household; a member who left, when the event's actor is
// the member; and otherwise a member the actor removed. It reports false for
// any other event.
func MembershipNotificationIn(ev eventstore.CloudEvent) (domain.NotificationCommand, bool, error) {
	switch ev.Type {
	case HouseholdMemberAddedType:
		var added domain.HouseholdMemberAdded
		if err := json.Unmarshal(ev.Data, &added); err != nil {
			return nil, false, fmt.Errorf("read %s %s: %w", ev.Type, ev.ID, err)
		}

		if added.InvitationID == uuid.Nil {
			return nil, false, nil
		}

		return domain.RaiseMemberJoined{MemberID: added.UserID}, true, nil
	case HouseholdMemberRemovedType:
		var removed domain.HouseholdMemberRemoved
		if err := json.Unmarshal(ev.Data, &removed); err != nil {
			return nil, false, fmt.Errorf("read %s %s: %w", ev.Type, ev.ID, err)
		}

		if ev.ActorID == removed.UserID {
			return domain.RaiseMemberLeft{MemberID: removed.UserID}, true, nil
		}

		return domain.RaiseMemberRemoved{MemberID: removed.UserID, RemovedBy: ev.ActorID}, true, nil
	default:
		return nil, false, nil
	}
}

// RaiseMembershipNotification raises the Notification a Household event
// brings, if any (MembershipNotificationIn), in tx, which is set to the
// event's Household.
func (r NotificationRaiser) RaiseMembershipNotification(ctx context.Context, tx *eventstore.Tx, ev eventstore.CloudEvent) error {
	raise, ok, err := MembershipNotificationIn(ev)
	if err != nil || !ok {
		return err
	}

	id, err := sharedkernel.NewID()
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	if _, err := r.notifications.Execute(ctx, tx, id, raise); err != nil {
		return fmt.Errorf("raise notification for %s %s: %w", ev.Type, ev.ID, err)
	}

	return nil
}
