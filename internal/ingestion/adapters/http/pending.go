package http

import (
	"context"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// The operations and fields below are in the contract but not implemented
// yet: each answers 501 until its ticket in .scratch/cash-accounts-and-transfers
// lands.

// ListTransferSuggestions implements listTransferSuggestions (ticket 07).
func (h *StatementImportHandler) ListTransferSuggestions(context.Context, oas.ListTransferSuggestionsParams,
) (oas.ListTransferSuggestionsRes, error) {
	return nil, httpapi.ErrNotImplemented
}

// matchPending reports whether an Accept asks to Match a leg the cash
// Account already has (ticket 07).
func matchPending(req oas.ImportedTransactionAcceptance) bool {
	return req.MatchTransactionID.Set
}
