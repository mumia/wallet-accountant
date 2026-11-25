package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// LastMemberType is for removing a Household's only member.
const LastMemberType = "/problems/last-member"

// RemoveMember implements removeMember: the caller removes a member, or
// leaves when the User ID is their own. A caller who isn't a member, or a
// User who isn't one, gets 404.
func (h *Handler) RemoveMember(ctx context.Context, params oas.RemoveMemberParams) (oas.RemoveMemberRes, error) {
	err := h.households.RemoveMember(ctx, params.HouseholdID, params.UserID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrMemberNotFound):
		return new(oas.RemoveMemberNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, app.ErrLastMember):
		p := httpapi.Problem(http.StatusConflict, LastMemberType, "Last member",
			"The only member of a Household can't leave it.")

		return new(oas.RemoveMemberConflict(p)), nil
	case err != nil:
		return nil, fmt.Errorf("remove member: %w", err)
	}

	return &oas.RemoveMemberNoContent{}, nil
}
