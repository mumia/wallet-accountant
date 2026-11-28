package http

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ingestion/app"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// ListTransferSuggestions implements listTransferSuggestions.
func (h *StatementImportHandler) ListTransferSuggestions(ctx context.Context, params oas.ListTransferSuggestionsParams,
) (oas.ListTransferSuggestionsRes, error) {
	var counterpart *uuid.UUID
	if id, ok := params.AccountID.Get(); ok {
		counterpart = &id
	}

	got, err := h.imported.TransferSuggestions(ctx, params.HouseholdID, params.ImportedTransactionID, counterpart)

	if refused, ok := errors.AsType[*app.TransferRefusedError](err); ok {
		return refusedTransfer(refused), nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrImportedTransactionNotFound):
		return new(oas.ListTransferSuggestionsNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list transfer suggestions: %w", err)
	}

	out := &oas.TransferSuggestionList{
		Legs:                 make([]oas.Transaction, 0, len(got.Legs)),
		ImportedTransactions: make([]oas.ImportedTransaction, 0, len(got.Rows)),
	}

	for _, r := range got.Rows {
		row, err := importedTransactionOut(r)
		if err != nil {
			return nil, err
		}

		out.ImportedTransactions = append(out.ImportedTransactions, row)
	}

	for _, l := range got.Legs {
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
