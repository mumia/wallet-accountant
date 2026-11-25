package http

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// NotificationHandler implements the Notification operations of
// oas.Handler.
type NotificationHandler struct {
	notifications *app.Notifications
}

// NewNotificationHandler returns the Notifications HTTP adapter.
func NewNotificationHandler(notifications *app.Notifications) *NotificationHandler {
	return &NotificationHandler{notifications: notifications}
}

// ListNotifications implements listNotifications.
func (h *NotificationHandler) ListNotifications(ctx context.Context, params oas.ListNotificationsParams) (oas.ListNotificationsRes, error) {
	list, err := h.notifications.List(ctx, params.HouseholdID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound):
		return new(oas.ListNotificationsNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	items := make([]oas.Notification, 0, len(list))
	for _, n := range list {
		items = append(items, *notificationResponse(n))
	}

	return &oas.NotificationList{Items: items}, nil
}

// MarkNotificationRead implements markNotificationRead.
func (h *NotificationHandler) MarkNotificationRead(ctx context.Context, params oas.MarkNotificationReadParams) (oas.MarkNotificationReadRes, error) {
	n, err := h.notifications.MarkRead(ctx, params.HouseholdID, params.NotificationID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrNotificationNotFound):
		return new(oas.MarkNotificationReadNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("mark notification read: %w", err)
	}

	return notificationResponse(n), nil
}

// notificationResponse leaves out the fields of other kinds.
func notificationResponse(n app.Notification) *oas.Notification {
	res := &oas.Notification{
		NotificationID: n.ID,
		Kind:           strings.ToUpper(string(n.Kind)),
		CreatedAt:      n.RaisedAt,
		Read:           n.Read,
	}

	optUUID := func(id uuid.UUID) oas.OptUUID {
		if id == uuid.Nil {
			return oas.OptUUID{}
		}

		return oas.NewOptUUID(id)
	}
	res.StatementImportID, res.ConnectionID, res.AccountID =
		optUUID(n.StatementImportID), optUUID(n.ConnectionID), optUUID(n.AccountID)

	if n.SuspectedDuplicates != 0 {
		res.SuspectedDuplicates = oas.NewOptInt32(int32(n.SuspectedDuplicates)) //nolint:gosec // a file's row count fits
	}

	if n.Member.ID != uuid.Nil {
		res.Member = oas.NewOptUserRef(userRefResponse(n.Member))
	}

	if n.RemovedBy.ID != uuid.Nil {
		res.RemovedBy = oas.NewOptUserRef(userRefResponse(n.RemovedBy))
	}

	return res
}
