package http

import (
	"context"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// The operations below are in the contract but not implemented yet: each
// answers 501 until its ticket in .scratch/join-household lands.

// CreateMembership implements createMembership (ticket 06).
func (h *Handler) CreateMembership(context.Context, *oas.MembershipCreation, oas.CreateMembershipParams,
) (oas.CreateMembershipRes, error) {
	return nil, httpapi.ErrNotImplemented
}

// ListMembers implements listMembers (ticket 08).
func (h *Handler) ListMembers(context.Context, oas.ListMembersParams) (oas.ListMembersRes, error) {
	return nil, httpapi.ErrNotImplemented
}

// RemoveMember implements removeMember (ticket 09).
func (h *Handler) RemoveMember(context.Context, oas.RemoveMemberParams) (oas.RemoveMemberRes, error) {
	return nil, httpapi.ErrNotImplemented
}
