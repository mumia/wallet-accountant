package http

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// counterpartNotCash is the Ledger's reason for suggestions on a statement
// Account until statement-transfers ticket 03 lands; the handler answers
// 501 for it, since the contract no longer has it.
const counterpartNotCash = "COUNTERPART_NOT_CASH"

// transferDetails explains each reason an Accept as Transfer is refused.
var transferDetails = map[string]string{
	"CATEGORY_AND_TRANSFER": "A Transfer leg is booked against Account transfer: send category_id or " +
		"transfer_account_id, not both.",
	"MATCH_WITHOUT_TRANSFER": "match_transaction_id and match_imported_transaction_id need transfer_account_id, " +
		"the Account they are on.",
	"MATCH_LEG_AND_ROW": "Send match_transaction_id or match_imported_transaction_id, not both: the other side is " +
		"either a leg already booked or a pending row.",
	"NOT_A_MATCHING_LEG": "match_transaction_id isn't an In transit leg on transfer_account_id naming this row's " +
		"Account with the opposite amount.",
	"ACCOUNT_KIND_NOT_SUPPORTED": "Loan and crypto wallet Accounts take no part in Transfers yet: only cash, bank " +
		"and credit card Accounts do.",
	"SAME_ACCOUNT":      "transfer_account_id is the row's own Account.",
	"CURRENCY_MISMATCH": "transfer_account_id has another Account currency than the row.",
	"UNKNOWN_ACCOUNT":   "transfer_account_id is not one of the Household's Accounts.",
	"BEFORE_START_DATE": "The row's date is before the cash Account's start date: the cash Account didn't exist " +
		"yet.",
}

// transferRefused is the 422 of a refused Accept as Transfer.
func transferRefused(reason string) *oas.TransferProblem {
	p := httpapi.Unprocessable(InvalidTransferType, "Invalid Transfer", transferDetails[reason])
	p.Reason = oas.NewOptString(reason)

	return p
}

// refusedTransfer is the 422 of an Accept as Transfer the Ledger refused.
// A row before the cash Account's start date names that Account and both
// dates.
func refusedTransfer(e *app.TransferRefusedError) *oas.TransferProblem {
	p := transferRefused(e.Reason)

	if !e.StartDate.IsZero() {
		p.Detail = oas.NewOptString(fmt.Sprintf("The row's date, %s, is before the cash Account's start date, %s: "+
			"the cash Account didn't exist yet. Accept the row against an Expense Category, such as cash spending, "+
			"or as a Transfer to a cash Account that started on or before the row's date.",
			e.Date.Format(time.DateOnly), e.StartDate.Format(time.DateOnly)))
	}

	if e.AccountID != uuid.Nil {
		p.AccountID = oas.NewOptUUID(e.AccountID)
	}

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

		if (v.MatchTransactionID.Set || v.MatchImportedTransactionID.Set) && !v.TransferAccountID.Set {
			return transferRefused("MATCH_WITHOUT_TRANSFER"), nil
		}

		if v.MatchTransactionID.Set && v.MatchImportedTransactionID.Set {
			return transferRefused("MATCH_LEG_AND_ROW"), nil
		}

		if v.MatchImportedTransactionID.Set {
			return nil, fmt.Errorf("%w: match_imported_transaction_id (statement-transfers ticket 04)",
				httpapi.ErrNotImplemented)
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
		return refusedTransfer(refused), nil
	}

	if refused, ok := errors.AsType[*app.CategoryDirectionError](err); ok && a.Category != nil {
		return httpapi.CategoryDirection(*a.Category, refused.Kind, refused.Direction), nil
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
		AccountID:             s.AccountID,
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
