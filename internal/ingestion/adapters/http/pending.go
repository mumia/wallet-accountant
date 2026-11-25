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

// transferPending reports whether an Accept asks for a Transfer (tickets 06
// and 07).
func transferPending(req oas.OptImportedTransactionAcceptance) bool {
	return req.Set && (req.Value.TransferAccountID.Set || req.Value.MatchTransactionID.Set)
}
