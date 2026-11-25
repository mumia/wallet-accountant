package http

import (
	"context"
	"errors"
	"fmt"

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
	// ImportedTransactionNotSuspectedDuplicateType is for keeping a row that
	// isn't a Suspected duplicate.
	ImportedTransactionNotSuspectedDuplicateType = "/problems/imported-transaction-not-suspected-duplicate"
	// ImportedTransactionSuspectedDuplicateType is for accepting a Suspected
	// duplicate before it is kept.
	ImportedTransactionSuspectedDuplicateType = "/problems/imported-transaction-suspected-duplicate"
	InternalCategoryType                      = "/problems/internal-category"
	InvalidCategoryType                       = "/problems/invalid-category"
	// InvalidTransferType is for an Accept as Transfer the Ledger refuses;
	// its reason says why.
	InvalidTransferType = "/problems/invalid-transfer"
)

// transferDetails explains each reason an Accept as Transfer is refused.
var transferDetails = map[string]string{
	"CATEGORY_AND_TRANSFER": "A Transfer leg is booked against Account transfer: send category_id or " +
		"transfer_account_id, not both.",
	"COUNTERPART_NOT_CASH": "transfer_account_id must be a cash Account: Transfers between statement Accounts " +
		"come later.",
	"MATCH_WITHOUT_TRANSFER": "match_transaction_id needs transfer_account_id, the cash Account it is on.",
	"NOT_A_MATCHING_LEG": "match_transaction_id isn't an In transit leg on the cash Account naming this row's " +
		"Account with the opposite amount.",
	"SAME_ACCOUNT":      "transfer_account_id is the row's own Account.",
	"CURRENCY_MISMATCH": "transfer_account_id has another Account currency than the row.",
	"UNKNOWN_ACCOUNT":   "transfer_account_id is not one of the Household's Accounts.",
}

// transferRefused is the 422 of a refused Accept as Transfer.
func transferRefused(reason string) *oas.TransferProblem {
	p := httpapi.Unprocessable(InvalidTransferType, "Invalid Transfer", transferDetails[reason])
	p.Reason = oas.NewOptString(reason)

	return p
}

var importedStatesOut = map[domain.ImportedState]string{
	domain.ImportedPending:            "PENDING",
	domain.ImportedSuspectedDuplicate: "SUSPECTED_DUPLICATE",
	domain.ImportedAccepted:           "ACCEPTED",
	domain.ImportedDiscarded:          "DISCARDED",
}

var importedStatesIn = map[string]domain.ImportedState{
	"PENDING":             domain.ImportedPending,
	"SUSPECTED_DUPLICATE": domain.ImportedSuspectedDuplicate,
	"ACCEPTED":            domain.ImportedAccepted,
	"DISCARDED":           domain.ImportedDiscarded,
}

// monthRefusal is the 409 of a row refused for its Booking month (closed,
// or not open yet), or nil for any other error.
func monthRefusal(err error) *oas.MonthProblem {
	e, ok := errors.AsType[*app.MonthError](err)

	switch {
	case !ok:
		return nil
	case errors.Is(e.Err, app.ErrMonthClosed):
		p := httpapi.MonthClosed(e.Month, e.OpenMonth)
		if e.AccountID != uuid.Nil {
			p.AccountID = oas.NewOptUUID(e.AccountID)
		}

		return p
	case errors.Is(e.Err, app.ErrMonthNotOpenYet):
		return httpapi.MonthNotOpenYet(e.Month, e.OpenMonth)
	default:
		return nil
	}
}

// AcceptImportedTransaction implements acceptImportedTransaction.
func (h *StatementImportHandler) AcceptImportedTransaction(ctx context.Context, req oas.OptImportedTransactionAcceptance,
	params oas.AcceptImportedTransactionParams,
) (oas.AcceptImportedTransactionRes, error) {
	var a app.Acceptance

	if req.Set {
		v := req.Value

		if v.CategoryID.Set && v.TransferAccountID.Set {
			return transferRefused("CATEGORY_AND_TRANSFER"), nil
		}

		if v.MatchTransactionID.Set && !v.TransferAccountID.Set {
			return transferRefused("MATCH_WITHOUT_TRANSFER"), nil
		}

		if id, ok := v.MatchTransactionID.Get(); ok {
			a.MatchLeg = &id
		}

		if id, ok := v.CategoryID.Get(); ok {
			a.Category = &id
		}

		if id, ok := v.TransferAccountID.Get(); ok {
			a.TransferAccount = &id
		}
	}

	accepted, err := h.imported.Accept(ctx, params.HouseholdID, params.ImportedTransactionID, a)
	if p := monthRefusal(err); p != nil {
		return p, nil
	}

	if refused, ok := errors.AsType[*app.TransferRefusedError](err); ok {
		return transferRefused(refused.Reason), nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrImportedTransactionNotFound),
		errors.Is(err, app.ErrAccountNotFound):
		return new(oas.AcceptImportedTransactionNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, app.ErrImportedTransactionAlreadyAccepted):
		return httpapi.MonthConflict(
			ImportedTransactionAlreadyAcceptedType, "Imported transaction already Accepted",
			"The Imported transaction is already booked in the Ledger.", "", ""), nil
	case errors.Is(err, app.ErrImportedTransactionDiscarded):
		return httpapi.MonthConflict(
			ImportedTransactionDiscardedType, "Imported transaction Discarded",
			"The Imported transaction is Discarded, which is final.", "", ""), nil
	case errors.Is(err, app.ErrImportedTransactionSuspectedDuplicate):
		return httpapi.MonthConflict(
			ImportedTransactionSuspectedDuplicateType, "Imported transaction is a Suspected duplicate",
			"Keep the Suspected duplicate before accepting it, or Discard it.", "", ""), nil
	case errors.Is(err, app.ErrInternalCategory):
		return httpapi.Unprocessable(InternalCategoryType, "Internal Category",
			"Internal Categories belong to the system: only Uncategorised can be chosen when accepting."), nil
	case errors.Is(err, app.ErrCategoryNotFound):
		return httpapi.Unprocessable(InvalidCategoryType, "Invalid Category",
			"The category_id is not one of the Household's Categories."), nil
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
	if p := monthRefusal(err); p != nil {
		return p, nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrImportedTransactionNotFound):
		return new(oas.DiscardImportedTransactionNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, app.ErrImportedTransactionAlreadyDiscarded):
		return httpapi.MonthConflict(
			ImportedTransactionAlreadyDiscardedType, "Imported transaction already Discarded",
			"The Imported transaction is already Discarded.", "", ""), nil
	case errors.Is(err, app.ErrImportedTransactionAlreadyAccepted):
		return httpapi.MonthConflict(
			ImportedTransactionAlreadyAcceptedType, "Imported transaction already Accepted",
			"The Imported transaction is booked in the Ledger: reverse its Transaction first.", "", ""), nil
	case err != nil:
		return nil, fmt.Errorf("discard imported transaction: %w", err)
	}

	out, err := importedTransactionOut(discarded)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// KeepImportedTransaction implements keepImportedTransaction.
func (h *StatementImportHandler) KeepImportedTransaction(ctx context.Context,
	params oas.KeepImportedTransactionParams,
) (oas.KeepImportedTransactionRes, error) {
	kept, err := h.imported.Keep(ctx, params.HouseholdID, params.ImportedTransactionID)
	if p := monthRefusal(err); p != nil {
		return p, nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrImportedTransactionNotFound):
		return new(oas.KeepImportedTransactionNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, app.ErrImportedTransactionNotSuspectedDuplicate):
		return httpapi.MonthConflict(
			ImportedTransactionNotSuspectedDuplicateType, "Not a Suspected duplicate",
			"Only a Suspected duplicate can be kept.", "", ""), nil
	case err != nil:
		return nil, fmt.Errorf("keep imported transaction: %w", err)
	}

	out, err := importedTransactionOut(kept)
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

	if r := i.Resembles; r != nil {
		resembled, err := resembledOut(*r)
		if err != nil {
			return oas.ImportedTransaction{}, err
		}

		item.Resembles = oas.NewOptResembledImportedTransaction(resembled)
	}

	return item, nil
}

// resembledOut is the earlier Imported transaction a Suspected duplicate
// resembles, as the API writes it.
func resembledOut(i app.ImportedTransaction) (oas.ResembledImportedTransaction, error) {
	s := i.State

	a, err := money(s.Amount, s.Currency)
	if err != nil {
		return oas.ResembledImportedTransaction{}, err
	}

	out := oas.ResembledImportedTransaction{
		ImportedTransactionID: i.ID,
		StatementImportID:     s.StatementImportID,
		State:                 importedStatesOut[s.State],
		BookingDate:           s.BookingDate,
		Amount:                a,
		Description:           s.Description,
	}

	if s.BankTransactionID != "" {
		out.BankTransactionID = oas.NewOptString(s.BankTransactionID)
	}

	return out, nil
}
