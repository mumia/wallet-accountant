package http

import (
	"context"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// The operations and fields below are in the contract but not implemented
// yet: each answers 501 until its ticket in .scratch/cash-accounts-and-transfers
// lands.

// transferLegPending reports whether a cash Transaction is a Transfer leg
// (tickets 07 and 09).
func transferLegPending(req *oas.CashTransactionCreation) bool {
	return req.CounterpartAccountID.Set
}

// GetMatch implements getMatch (ticket 05).
func (h *TransactionHandler) GetMatch(context.Context, oas.GetMatchParams) (oas.GetMatchRes, error) {
	return nil, httpapi.ErrNotImplemented
}

// inTransitPending reports whether a Transaction list asks for In transit
// legs only (ticket 05).
func inTransitPending(params oas.ListAccountTransactionsParams) bool {
	return params.InTransit.Or(false)
}
