package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ingestion/app"
	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// Problem types of accepting an Imported transaction. The Category ones are
// the Ledger's, as creating a Category answers them.
const (
	ImportedTransactionAlreadyAcceptedType = "/problems/imported-transaction-already-accepted"
	ImportedTransactionDiscardedType       = "/problems/imported-transaction-discarded"
	// ImportedTransactionAlreadyDiscardedType is for discarding twice.
	ImportedTransactionAlreadyDiscardedType = "/problems/imported-transaction-already-discarded"
	InternalCategoryType                    = "/problems/internal-category"
	InvalidCategoryType                     = "/problems/invalid-category"
)

var importedStatesOut = map[domain.ImportedState]string{
	domain.ImportedPending:   "PENDING",
	domain.ImportedAccepted:  "ACCEPTED",
	domain.ImportedDiscarded: "DISCARDED",
}

var importedStatesIn = map[string]domain.ImportedState{
	"PENDING":   domain.ImportedPending,
	"ACCEPTED":  domain.ImportedAccepted,
	"DISCARDED": domain.ImportedDiscarded,
}

// AcceptImportedTransaction implements acceptImportedTransaction.
func (h *StatementImportHandler) AcceptImportedTransaction(ctx context.Context, req oas.OptImportedTransactionAcceptance,
	params oas.AcceptImportedTransactionParams,
) (oas.AcceptImportedTransactionRes, error) {
	var category *uuid.UUID
	if id, ok := req.Value.CategoryID.Get(); req.Set && ok {
		category = &id
	}

	accepted, err := h.imported.Accept(ctx, params.HouseholdID, params.ImportedTransactionID, category)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrImportedTransactionNotFound),
		errors.Is(err, app.ErrAccountNotFound):
		return new(oas.AcceptImportedTransactionNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, app.ErrImportedTransactionAlreadyAccepted):
		return new(oas.AcceptImportedTransactionConflict(httpapi.Problem(http.StatusConflict,
			ImportedTransactionAlreadyAcceptedType, "Imported transaction already Accepted",
			"The Imported transaction is already booked in the Ledger."))), nil
	case errors.Is(err, app.ErrImportedTransactionDiscarded):
		return new(oas.AcceptImportedTransactionConflict(httpapi.Problem(http.StatusConflict,
			ImportedTransactionDiscardedType, "Imported transaction Discarded",
			"The Imported transaction is Discarded, which is final."))), nil
	case errors.Is(err, app.ErrInternalCategory):
		return new(oas.AcceptImportedTransactionUnprocessableEntity(httpapi.Problem(http.StatusUnprocessableEntity,
			InternalCategoryType, "Internal Category",
			"Internal Categories belong to the system: only Uncategorised can be chosen when accepting."))), nil
	case errors.Is(err, app.ErrCategoryNotFound):
		return new(oas.AcceptImportedTransactionUnprocessableEntity(httpapi.Problem(http.StatusUnprocessableEntity,
			InvalidCategoryType, "Invalid Category", "The category_id is not one of the Household's Categories."))), nil
	case err != nil:
		return nil, fmt.Errorf("accept imported transaction: %w", err)
	}

	out, err := importedTransactionOut(accepted)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// DiscardImportedTransaction implements discardImportedTransaction.
func (h *StatementImportHandler) DiscardImportedTransaction(ctx context.Context,
	params oas.DiscardImportedTransactionParams,
) (oas.DiscardImportedTransactionRes, error) {
	discarded, err := h.imported.Discard(ctx, params.HouseholdID, params.ImportedTransactionID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrImportedTransactionNotFound):
		return new(oas.DiscardImportedTransactionNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, app.ErrImportedTransactionAlreadyDiscarded):
		return new(oas.DiscardImportedTransactionConflict(httpapi.Problem(http.StatusConflict,
			ImportedTransactionAlreadyDiscardedType, "Imported transaction already Discarded",
			"The Imported transaction is already Discarded."))), nil
	case errors.Is(err, app.ErrImportedTransactionAlreadyAccepted):
		return new(oas.DiscardImportedTransactionConflict(httpapi.Problem(http.StatusConflict,
			ImportedTransactionAlreadyAcceptedType, "Imported transaction already Accepted",
			"The Imported transaction is booked in the Ledger: reverse its Transaction first."))), nil
	case err != nil:
		return nil, fmt.Errorf("discard imported transaction: %w", err)
	}

	out, err := importedTransactionOut(discarded)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// importedTransactionOut is an Imported transaction as the API writes it.
func importedTransactionOut(i app.ImportedTransaction) (oas.ImportedTransaction, error) {
	s := i.State

	a, err := money(s.Amount, s.Currency)
	if err != nil {
		return oas.ImportedTransaction{}, err
	}

	item := oas.ImportedTransaction{
		ImportedTransactionID: i.ID,
		StatementImportID:     s.StatementImportID,
		State:                 importedStatesOut[s.State],
		Line:                  int32(s.Line), //nolint:gosec // a statement file is far smaller
		BookingDate:           s.BookingDate,
		TransactionDate:       s.TransactionDate,
		Amount:                a,
		Description:           s.Description,
	}

	if !s.ValueDate.IsZero() {
		item.ValueDate = oas.NewOptDate(s.ValueDate)
	}

	if s.Payee != "" {
		item.Payee = oas.NewOptString(s.Payee)
	}

	if s.BankTransactionID != "" {
		item.BankTransactionID = oas.NewOptString(s.BankTransactionID)
	}

	if s.TransactionID != uuid.Nil {
		item.TransactionID = oas.NewOptUUID(s.TransactionID)
	}

	return item, nil
}
