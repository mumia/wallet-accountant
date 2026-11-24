// Package http is the Households HTTP adapter: the ogen operations for Users
// and Households.
package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
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
	// InvalidHouseholdType is for a Household the domain refuses to create.
	InvalidHouseholdType = "/problems/invalid-household"
	// InvalidUserType is for a display name the domain refuses: one of only
	// spaces.
	InvalidUserType = "/problems/invalid-user"
)

// errNoIdentity means the security handler did not run; it should never happen.
var errNoIdentity = errors.New("request has no authenticated identity")

// Handler implements the Households operations of oas.Handler.
type Handler struct {
	users      *app.Users
	households *app.Households
}

// NewHandler returns the Households HTTP adapter.
func NewHandler(users *app.Users, households *app.Households) *Handler {
	return &Handler{users: users, households: households}
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
		return new(oas.GetMeForbidden(registrationRequired())), nil
	case err != nil:
		return nil, fmt.Errorf("get me: %w", err)
	}

	return meResponse(user), nil
}

// ChangeMe implements changeMe.
func (h *Handler) ChangeMe(ctx context.Context, req *oas.MeChange) (oas.ChangeMeRes, error) {
	id, ok := authn.IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}

	user, err := h.users.Rename(ctx, id, string(req.DisplayName))

	switch {
	case errors.Is(err, app.ErrRegistrationRequired):
		return new(oas.ChangeMeForbidden(registrationRequired())), nil
	case errors.Is(err, domain.ErrDisplayNameRequired):
		return new(oas.ChangeMeUnprocessableEntity(blankDisplayName())), nil
	case err != nil:
		return nil, fmt.Errorf("change me: %w", err)
	}

	return meResponse(user), nil
}

// CreateRegistration implements createRegistration.
func (h *Handler) CreateRegistration(ctx context.Context, req *oas.Registration, _ oas.CreateRegistrationParams) (oas.CreateRegistrationRes, error) {
	id, ok := authn.IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}

	user, err := h.users.Register(ctx, id, string(req.InvitationCode), string(req.DisplayName))

	switch {
	case errors.Is(err, app.ErrInvalidInvitation):
		p := httpapi.Problem(http.StatusUnprocessableEntity, InvalidInvitationType, "Invalid Invitation code",
			"The Invitation code is unknown, expired, superseded or already used.")

		return new(oas.CreateRegistrationUnprocessableEntity(p)), nil
	case errors.Is(err, app.ErrAlreadyRegistered):
		p := httpapi.Problem(http.StatusConflict, AlreadyRegisteredType, "Already registered",
			"This identity is already a User.")

		return new(oas.CreateRegistrationConflict(p)), nil
	case errors.Is(err, domain.ErrDisplayNameRequired):
		return new(oas.CreateRegistrationUnprocessableEntity(blankDisplayName())), nil
	case err != nil:
		return nil, fmt.Errorf("create registration: %w", err)
	}

	return meResponse(user), nil
}

func meResponse(u app.User) *oas.Me {
	me := &oas.Me{UserID: u.ID, IsInstanceAdministrator: u.IsInstanceAdministrator}
	if u.DisplayName != "" {
		me.DisplayName = oas.NewOptDisplayNameOut(oas.DisplayNameOut(u.DisplayName))
	}

	return me
}

// CreateHousehold implements createHousehold.
func (h *Handler) CreateHousehold(ctx context.Context, req *oas.HouseholdCreation, _ oas.CreateHouseholdParams) (oas.CreateHouseholdRes, error) {
	id, ok := authn.IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}

	created, err := h.households.Create(ctx, id, req.Name, string(req.BaseCurrency))

	switch {
	case errors.Is(err, app.ErrRegistrationRequired):
		return new(oas.CreateHouseholdForbidden(registrationRequired())), nil
	case errors.Is(err, domain.ErrHouseholdNameRequired):
		p := httpapi.Problem(http.StatusUnprocessableEntity, InvalidHouseholdType, "Invalid Household",
			"The name must not be blank.")

		return new(oas.CreateHouseholdUnprocessableEntity(p)), nil
	case errors.Is(err, sharedkernel.ErrUnknownCurrency):
		p := httpapi.Problem(http.StatusUnprocessableEntity, InvalidHouseholdType, "Invalid Household",
			"The Base currency is not a known currency.")

		return new(oas.CreateHouseholdUnprocessableEntity(p)), nil
	case err != nil:
		return nil, fmt.Errorf("create household: %w", err)
	}

	// The response holds the new Household's data, so a replay of it is
	// stored in that Household's scope.
	httpapi.ResponseBelongsTo(ctx, created.ID)

	return householdResponse(created), nil
}

// ListHouseholds implements listHouseholds.
func (h *Handler) ListHouseholds(ctx context.Context) (oas.ListHouseholdsRes, error) {
	id, ok := authn.IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}

	list, err := h.households.List(ctx, id)

	switch {
	case errors.Is(err, app.ErrRegistrationRequired):
		return new(oas.ListHouseholdsForbidden(registrationRequired())), nil
	case err != nil:
		return nil, fmt.Errorf("list households: %w", err)
	}

	items := make([]oas.Household, 0, len(list))
	for _, hh := range list {
		items = append(items, *householdResponse(hh))
	}

	return &oas.HouseholdList{Items: items}, nil
}

// GetHousehold implements getHousehold.
func (h *Handler) GetHousehold(ctx context.Context, params oas.GetHouseholdParams) (oas.GetHouseholdRes, error) {
	got, err := h.households.Get(ctx, params.HouseholdID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound):
		return new(oas.GetHouseholdNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("get household: %w", err)
	}

	return householdResponse(got), nil
}

func householdResponse(h app.Household) *oas.Household {
	return &oas.Household{HouseholdID: h.ID, Name: h.Name, BaseCurrency: oas.CurrencyCode(h.BaseCurrency)}
}

func blankDisplayName() oas.Problem {
	return httpapi.Problem(http.StatusUnprocessableEntity, InvalidUserType, "Invalid User",
		"The display name must not be blank.")
}

func registrationRequired() oas.Problem {
	return httpapi.Problem(http.StatusForbidden, RegistrationRequiredType, "Registration required",
		"Redeem an Invitation to become a User.")
}
