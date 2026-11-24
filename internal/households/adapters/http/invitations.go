package http

import (
	"context"
	"errors"
	"fmt"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// InvitationHandler implements the Invitation operations of oas.Handler.
type InvitationHandler struct {
	invitations *app.Invitations
}

// NewInvitationHandler returns the Invitations HTTP adapter.
func NewInvitationHandler(invitations *app.Invitations) *InvitationHandler {
	return &InvitationHandler{invitations: invitations}
}

// CreateInvitation implements createInvitation. The code is in this
// response only; a replay with the same Idempotency-Key returns it again
// from the stored response, kept in the Household's scope.
func (h *InvitationHandler) CreateInvitation(ctx context.Context, params oas.CreateInvitationParams) (oas.CreateInvitationRes, error) {
	issued, err := h.invitations.Issue(ctx, params.HouseholdID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound):
		return new(oas.CreateInvitationNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	return &oas.IssuedInvitation{InvitationID: issued.ID, Code: issued.Code, ExpiresAt: issued.ExpiresAt}, nil
}

// ListInvitations implements listInvitations.
func (h *InvitationHandler) ListInvitations(ctx context.Context, params oas.ListInvitationsParams) (oas.ListInvitationsRes, error) {
	list, err := h.invitations.ListOpen(ctx, params.HouseholdID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound):
		return new(oas.ListInvitationsNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list invitations: %w", err)
	}

	items := make([]oas.Invitation, 0, len(list))
	for _, inv := range list {
		items = append(items, oas.Invitation{
			InvitationID: inv.ID,
			IssuedBy:     userRefResponse(inv.IssuedBy),
			IssuedAt:     inv.IssuedAt,
			ExpiresAt:    inv.ExpiresAt,
		})
	}

	return &oas.InvitationList{Items: items}, nil
}

// userRefResponse leaves display_name out for a User who has none.
func userRefResponse(u app.UserRef) oas.UserRef {
	ref := oas.UserRef{UserID: u.ID}
	if u.DisplayName != "" {
		ref.DisplayName = oas.NewOptDisplayNameOut(oas.DisplayNameOut(u.DisplayName))
	}

	return ref
}
