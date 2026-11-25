package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mumia/wallet-accountant/internal/ledger/app"
	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// InvalidCursorType is the problem type for a page cursor the API didn't
// hand out.
const InvalidCursorType = "/problems/invalid-cursor"

// DefaultPageSize is how many items a page holds when the request doesn't
// say.
const DefaultPageSize = 50

// TransactionHandler implements the Transaction operations of oas.Handler.
type TransactionHandler struct {
	transactions *app.Transactions
}

// NewTransactionHandler returns the Transaction operations of the Ledger
// HTTP adapter.
func NewTransactionHandler(transactions *app.Transactions) *TransactionHandler {
	return &TransactionHandler{transactions: transactions}
}

// ListAccountTransactions implements listAccountTransactions.
func (h *TransactionHandler) ListAccountTransactions(ctx context.Context, params oas.ListAccountTransactionsParams,
) (oas.ListAccountTransactionsRes, error) {
	if inTransitPending(params) {
		return nil, httpapi.ErrNotImplemented
	}

	filter := app.TransactionFilter{Limit: int(params.Limit.Or(DefaultPageSize))}

	if m, ok := params.BookingMonth.Get(); ok {
		// The contract's pattern already checked it.
		month, err := time.Parse(domain.MonthLayout, m)
		if err != nil {
			return new(oas.ListAccountTransactionsBadRequest(httpapi.Problem(http.StatusBadRequest, "", "",
				"The booking_month must be written YYYY-MM."))), nil
		}

		filter.Month = month
	}

	if c, ok := params.Cursor.Get(); ok {
		cursor, err := app.ParseTransactionCursor(c)
		if err != nil {
			return new(oas.ListAccountTransactionsBadRequest(httpapi.Problem(http.StatusBadRequest, InvalidCursorType,
				"Invalid cursor", "The cursor is not one this API handed out."))), nil
		}

		filter.After = &cursor
	}

	page, err := h.transactions.List(ctx, params.HouseholdID, params.AccountID, filter)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.ListAccountTransactionsNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list account transactions: %w", err)
	}

	out := &oas.AccountTransactionPage{Items: make([]oas.AccountTransaction, 0, len(page.Items))}

	for _, t := range page.Items {
		amount, err := money(t.Amount, t.Currency)
		if err != nil {
			return nil, err
		}

		out.Items = append(out.Items, oas.AccountTransaction{
			TransactionID:   t.TransactionID,
			Amount:          amount,
			BookingDate:     t.BookingDate,
			TransactionDate: t.TransactionDate,
			Description:     t.Description,
			CategoryID:      t.CategoryID,
			CategoryName:    t.CategoryName,

			ReversalOfTransactionID: optionalID(t.ReversalOf),
			ReversedByTransactionID: optionalID(t.ReversedBy),
			ImportedTransactionID:   optionalID(t.ImportedTransactionID),
		})
	}

	if page.Next != nil {
		out.NextCursor = oas.NewOptString(page.Next.String())
	}

	return out, nil
}
