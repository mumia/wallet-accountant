// Package http is the Households HTTP adapter: the ogen operations for Users
// and Households.
package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// Problem types of the Households operations.
const (
	// RegistrationRequiredType is for a verified identity that is not a User yet.
	RegistrationRequiredType = "/problems/registration-required"
	// InvalidInvitationType is for any Invitation code that can't be
	// redeemed: unknown, expired, superseded or already used alike.
	InvalidInvitationType = "/problems/invalid-invitation"
	// AlreadyRegisteredType is for an identity that is already a User.
	AlreadyRegisteredType = "/problems/already-registered"
)

// errNoIdentity means the security handler did not run; it should never happen.
var errNoIdentity = errors.New("request has no authenticated identity")

// Handler implements the Households operations of oas.Handler.
type Handler struct {
	users *app.Users
}

// NewHandler returns the Households HTTP adapter.
func NewHandler(users *app.Users) *Handler {
	return &Handler{users: users}
}

// GetMe implements getMe.
func (h *Handler) GetMe(ctx context.Context) (oas.GetMeRes, error) {
	id, ok := authn.IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}

	user, err := h.users.Me(ctx, id)

	switch {
	case errors.Is(err, app.ErrRegistrationRequired):
		p := httpapi.Problem(http.StatusForbidden, RegistrationRequiredType, "Registration required",
			"Redeem an Invitation to become a User.")

		return new(oas.GetMeForbidden(p)), nil
	case err != nil:
		return nil, fmt.Errorf("get me: %w", err)
	}

	return &oas.Me{UserID: user.ID, IsInstanceAdministrator: user.IsInstanceAdministrator}, nil
}

// CreateRegistration implements createRegistration.
func (h *Handler) CreateRegistration(ctx context.Context, req *oas.Registration) (oas.CreateRegistrationRes, error) {
	id, ok := authn.IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}

	user, err := h.users.Register(ctx, id, req.InvitationCode)

	switch {
	case errors.Is(err, app.ErrInvalidInvitation):
		p := httpapi.Problem(http.StatusUnprocessableEntity, InvalidInvitationType, "Invalid Invitation code",
			"The Invitation code is unknown, expired, superseded or already used.")

		return new(oas.CreateRegistrationUnprocessableEntity(p)), nil
	case errors.Is(err, app.ErrAlreadyRegistered):
		p := httpapi.Problem(http.StatusConflict, AlreadyRegisteredType, "Already registered",
			"This identity is already a User.")

		return new(oas.CreateRegistrationConflict(p)), nil
	case err != nil:
		return nil, fmt.Errorf("create registration: %w", err)
	}

	return &oas.Me{UserID: user.ID, IsInstanceAdministrator: user.IsInstanceAdministrator}, nil
}
