package http

import (
	"context"
	"errors"
	"fmt"

	"github.com/mumia/wallet-accountant/internal/ingestion/app"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// ListTransferSuggestions implements listTransferSuggestions.
func (h *StatementImportHandler) ListTransferSuggestions(ctx context.Context, params oas.ListTransferSuggestionsParams,
) (oas.ListTransferSuggestionsRes, error) {
	account, ok := params.AccountID.Get()
	if !ok {
		return nil, fmt.Errorf("%w: suggestions without account_id (statement-transfers ticket 03)",
			httpapi.ErrNotImplemented)
	}

	legs, err := h.imported.TransferSuggestions(ctx, params.HouseholdID, params.ImportedTransactionID, account)

	if refused, ok := errors.AsType[*app.TransferRefusedError](err); ok {
		if refused.Reason == counterpartNotCash {
			return nil, fmt.Errorf("%w: suggestions on a statement Account (statement-transfers ticket 03)",
				httpapi.ErrNotImplemented)
		}

		return transferRefused(refused.Reason), nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrImportedTransactionNotFound):
		return new(oas.ListTransferSuggestionsNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list transfer suggestions: %w", err)
	}

	out := &oas.TransferSuggestionList{
		Legs:                 make([]oas.Transaction, 0, len(legs)),
		ImportedTransactions: []oas.ImportedTransaction{},
	}

	for _, l := range legs {
		amount, err := money(l.Amount, l.Currency)
		if err != nil {
			return nil, err
		}

		t := oas.Transaction{
			TransactionID:        l.TransactionID,
			AccountID:            l.AccountID,
			Amount:               amount,
			BookingDate:          l.BookingDate,
			TransactionDate:      l.TransactionDate,
			Description:          l.Description,
			CategoryID:           l.CategoryID,
			CategoryName:         l.CategoryName,
			CounterpartAccountID: oas.NewOptUUID(l.CounterpartAccountID),
		}

		if l.Payee != "" {
			t.Payee = oas.NewOptString(l.Payee)
		}

		out.Legs = append(out.Legs, t)
	}

	return out, nil
}
