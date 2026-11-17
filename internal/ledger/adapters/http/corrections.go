package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ledger/app"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// Problem types of correcting a Transaction. The Category ones are those
// of the Category operations.
const (
	OpeningBalanceTransactionType  = "/problems/opening-balance-transaction"
	TransactionIsAReversalType     = "/problems/transaction-is-a-reversal"
	TransactionAlreadyReversedType = "/problems/transaction-already-reversed"
	TransactionReversedType        = "/problems/transaction-reversed"
)

// correction is how a refused correction is answered: 404, or a problem
// whose status says whether it is a conflict.
type correction struct {
	notFound bool
	problem  oas.Problem
}

// correctionRefusal answers a refused correction; false for any other
// error.
func correctionRefusal(err error) (correction, bool) {
	refused := func(status int, typ, title, detail string) (correction, bool) {
		return correction{problem: httpapi.Problem(status, typ, title, detail)}, true
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrTransactionNotFound):
		return correction{notFound: true}, true
	case errors.Is(err, app.ErrTransactionAlreadyReversed):
		return refused(http.StatusConflict, TransactionAlreadyReversedType, "Transaction already reversed",
			"The Transaction is already reversed.")
	case errors.Is(err, app.ErrTransactionReversed):
		return refused(http.StatusConflict, TransactionReversedType, "Transaction reversed",
			"A reversed Transaction keeps its Category.")
	case errors.Is(err, app.ErrOpeningBalanceTransaction):
		return refused(http.StatusUnprocessableEntity, OpeningBalanceTransactionType, "Opening balance Transaction",
			"The Opening balance Transaction changes only by voiding the first Statement import.")
	case errors.Is(err, app.ErrTransactionIsAReversal):
		return refused(http.StatusUnprocessableEntity, TransactionIsAReversalType, "Transaction is a reversal",
			"A reversal can't be reversed or recategorised.")
	case errors.Is(err, app.ErrInternalCategory):
		return refused(http.StatusUnprocessableEntity, InternalCategoryType, "Internal Category",
			"Internal Categories belong to the system: only Uncategorised can be chosen.")
	case errors.Is(err, app.ErrCategoryNotFound):
		return refused(http.StatusUnprocessableEntity, InvalidCategoryType, "Invalid Category",
			"The category_id is not one of the Household's Categories.")
	default:
		return correction{}, false
	}
}

// ChangeTransaction implements changeTransaction.
func (h *TransactionHandler) ChangeTransaction(ctx context.Context, req *oas.TransactionChange,
	params oas.ChangeTransactionParams,
) (oas.ChangeTransactionRes, error) {
	d, err := h.transactions.SetCategory(ctx, params.HouseholdID, params.TransactionID, req.CategoryID)
	if c, ok := correctionRefusal(err); ok {
		switch {
		case c.notFound:
			return new(oas.ChangeTransactionNotFound(httpapi.NotFoundProblem())), nil
		case c.problem.Status == http.StatusConflict:
			return new(oas.ChangeTransactionConflict(c.problem)), nil
		default:
			return new(oas.ChangeTransactionUnprocessableEntity(c.problem)), nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("change transaction: %w", err)
	}

	return transactionOut(d)
}

// ReverseTransaction implements reverseTransaction.
func (h *TransactionHandler) ReverseTransaction(ctx context.Context, params oas.ReverseTransactionParams,
) (oas.ReverseTransactionRes, error) {
	d, err := h.transactions.Reverse(ctx, params.HouseholdID, params.TransactionID)
	if c, ok := correctionRefusal(err); ok {
		switch {
		case c.notFound:
			return new(oas.ReverseTransactionNotFound(httpapi.NotFoundProblem())), nil
		case c.problem.Status == http.StatusConflict:
			return new(oas.ReverseTransactionConflict(c.problem)), nil
		default:
			return new(oas.ReverseTransactionUnprocessableEntity(c.problem)), nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("reverse transaction: %w", err)
	}

	return transactionOut(d)
}

// transactionOut is a Transaction as the API writes it.
func transactionOut(d app.TransactionDetail) (*oas.Transaction, error) {
	amount, err := money(d.Amount, d.Currency)
	if err != nil {
		return nil, err
	}

	out := &oas.Transaction{
		TransactionID:           d.ID,
		AccountID:               d.AccountID,
		Amount:                  amount,
		BookingDate:             d.BookingDate,
		TransactionDate:         d.TransactionDate,
		Description:             d.Description,
		CategoryID:              d.CategoryID,
		CategoryName:            d.CategoryName,
		ImportedTransactionID:   optionalID(d.ImportedTransactionID),
		ReversalOfTransactionID: optionalID(d.ReversalOf),
		ReversedByTransactionID: optionalID(d.ReversedBy),
	}

	if !d.ValueDate.IsZero() {
		out.ValueDate = oas.NewOptDate(d.ValueDate)
	}

	if d.Payee != "" {
		out.Payee = oas.NewOptString(d.Payee)
	}

	return out, nil
}

// optionalID leaves out the zero ID.
func optionalID(id uuid.UUID) oas.OptUUID {
	if id == uuid.Nil {
		return oas.OptUUID{}
	}

	return oas.NewOptUUID(id)
}
