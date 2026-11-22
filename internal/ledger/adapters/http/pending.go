package http

import (
	"context"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// The operations below are in the contract but not implemented yet: each
// answers 501 until its ticket in .scratch/ui-first-slice-backend lands.

// ChangeAccount implements changeAccount (ticket 13).
func (h *AccountHandler) ChangeAccount(context.Context, *oas.AccountChange, oas.ChangeAccountParams,
) (oas.ChangeAccountRes, error) {
	return nil, httpapi.ErrNotImplemented
}
