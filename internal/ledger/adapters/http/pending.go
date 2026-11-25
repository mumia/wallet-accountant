package http

import (
	"context"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// The operations and fields below are in the contract but not implemented
// yet: each answers 501 until its ticket in .scratch/cash-accounts-and-transfers
// lands.

// RecordCashTransaction implements recordCashTransaction (tickets 03, 07
// and 09).
func (h *TransactionHandler) RecordCashTransaction(context.Context, *oas.CashTransactionCreation,
	oas.RecordCashTransactionParams,
) (oas.RecordCashTransactionRes, error) {
	return nil, httpapi.ErrNotImplemented
}

// GetMatch implements getMatch (ticket 05).
func (h *TransactionHandler) GetMatch(context.Context, oas.GetMatchParams) (oas.GetMatchRes, error) {
	return nil, httpapi.ErrNotImplemented
}

// cashClosePending reports whether a Month close sends a Cash count's date
// (ticket 04).
func cashClosePending(req *oas.MonthClosing) bool {
	return req.CountedOn.Set
}

// inTransitPending reports whether a Transaction list asks for In transit
// legs only (ticket 05).
func inTransitPending(params oas.ListAccountTransactionsParams) bool {
	return params.InTransit.Or(false)
}
