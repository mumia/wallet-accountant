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

// RegistrationRequiredType is the problem type for a verified identity that
// is not a User yet.
const RegistrationRequiredType = "/problems/registration-required"

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
