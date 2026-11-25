package http

import (
	"context"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// The operations below are in the contract but not implemented yet: each
// answers 501 until its ticket in .scratch/join-household lands.

// RemoveMember implements removeMember (ticket 09).
func (h *Handler) RemoveMember(context.Context, oas.RemoveMemberParams) (oas.RemoveMemberRes, error) {
	return nil, httpapi.ErrNotImplemented
}
