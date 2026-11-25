package http

import (
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
