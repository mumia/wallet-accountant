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

// AlreadyAMemberType is for redeeming an Invitation to a Household the User
// already belongs to.
const AlreadyAMemberType = "/problems/already-a-member"

// CreateMembership implements createMembership: Join Household for a User.
func (h *Handler) CreateMembership(ctx context.Context, req *oas.MembershipCreation, _ oas.CreateMembershipParams,
) (oas.CreateMembershipRes, error) {
	id, ok := authn.IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}

	joined, err := h.users.Join(ctx, id, string(req.InvitationCode))

	switch {
	case errors.Is(err, app.ErrRegistrationRequired):
		return new(oas.CreateMembershipForbidden(registrationRequired())), nil
	case errors.Is(err, app.ErrInvalidInvitation):
		return new(oas.CreateMembershipUnprocessableEntity(invalidInvitation())), nil
	case errors.Is(err, app.ErrAlreadyAMember):
		p := httpapi.Problem(http.StatusConflict, AlreadyAMemberType, "Already a member",
			"You are already a member of this Household. The Invitation code is still unused.")

		return new(oas.CreateMembershipConflict(p)), nil
	case err != nil:
		return nil, fmt.Errorf("create membership: %w", err)
	}

	// The response holds the Household's data, so a replay of it is stored
	// in that Household's scope.
	httpapi.ResponseBelongsTo(ctx, joined.ID)

	return householdResponse(joined), nil
}
