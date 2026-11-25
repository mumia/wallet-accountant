package http

import (
	"context"
	"errors"
	"fmt"

	"github.com/mumia/wallet-accountant/internal/ledger/app"
	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// Problem types of recording a cash Transaction.
const (
	// InvalidTransactionType is for a cash Transaction the Ledger refuses:
	// a zero amount, one in another currency or too precise, or a date
	// after today or before the start date.
	InvalidTransactionType = "/problems/invalid-transaction"
	// InvalidTransferType is for a Transfer the Ledger refuses; its reason
	// says why.
	InvalidTransferType = "/problems/invalid-transfer"
)

// TransferReasonCategoryAndTransfer is the reason of a Transfer sent with a
// Category too.
const TransferReasonCategoryAndTransfer = "CATEGORY_AND_TRANSFER"

// transferDetails explains each reason a Transfer is refused.
var transferDetails = map[app.TransferReason]string{
	app.TransferSameAccount:        "The counterpart is the Transaction's own Account.",
	app.TransferCurrencyMatch:      "The counterpart has another Account currency.",
	app.TransferUnknownAccount:     "The counterpart is not one of the Household's Accounts.",
	app.TransferNotAMatchingLeg:    "The legs aren't the two sides of one Transfer.",
	app.TransferCounterpartNotCash: "Transfers between statement Accounts come later.",
}

// transferRefused is the 422 of a refused Transfer.
func transferRefused(e *app.TransferRefusedError) *oas.TransferProblem {
	p := httpapi.Unprocessable(InvalidTransferType, "Invalid Transfer", transferDetails[e.Reason])
	p.Reason = oas.NewOptString(string(e.Reason))

	return p
}

// RecordCashTransaction implements recordCashTransaction. A Transfer leg
// naming another cash Account answers 501 until ticket 09.
func (h *TransactionHandler) RecordCashTransaction(ctx context.Context, req *oas.CashTransactionCreation,
	params oas.RecordCashTransactionParams,
) (oas.RecordCashTransactionRes, error) {
	if req.CategoryID.Set && req.CounterpartAccountID.Set {
		p := httpapi.Unprocessable(InvalidTransferType, "Invalid Transfer",
			"A Transfer leg is booked against Account transfer: send category_id or counterpart_account_id, not both.")
		p.Reason = oas.NewOptString(TransferReasonCategoryAndTransfer)

		return p, nil
	}

	c := app.CashTransaction{
		Account:     params.AccountID,
		Date:        req.Date,
		Amount:      req.Amount.Amount.Value,
		Currency:    string(req.Amount.Currency),
		Description: req.Description,
		Payee:       req.Payee.Or(""),
	}

	if id, ok := req.CategoryID.Get(); ok {
		c.Category = &id
	}

	if id, ok := req.CounterpartAccountID.Get(); ok {
		c.Counterpart = &id
	}

	d, err := h.transactions.RecordCash(ctx, params.HouseholdID, c)
	if errors.Is(err, app.ErrCashToCashNotBuilt) {
		return nil, httpapi.ErrNotImplemented
	}

	if refused, ok := errors.AsType[*app.TransferRefusedError](err); ok {
		return transferRefused(refused), nil
	}

	invalid := func(typ, title, detail string) (oas.RecordCashTransactionRes, error) {
		return httpapi.Unprocessable(typ, title, detail), nil
	}

	monthErr, _ := errors.AsType[*domain.MonthError](err)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.RecordCashTransactionNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, app.ErrMonthClosed) && monthErr != nil:
		p := httpapi.MonthClosed(monthErr.Month, monthErr.OpenMonth)
		p.AccountID = oas.NewOptUUID(params.AccountID)

		return p, nil
	case errors.Is(err, app.ErrNotACashAccount):
		return invalid(NotACashAccountType, "Not a cash Account",
			"Only a cash Account takes Transactions by hand: a statement Account takes them only from its statements.")
	case errors.Is(err, app.ErrZeroAmount):
		return invalid(InvalidTransactionType, "Invalid Transaction", "The amount must not be zero.")
	case errors.Is(err, app.ErrCurrencyMismatch):
		return invalid(InvalidTransactionType, "Invalid Transaction", "The amount must be in the Account currency.")
	case errors.Is(err, domain.ErrTooManyDecimalPlaces):
		return invalid(InvalidTransactionType, "Invalid Transaction",
			"The amount has more decimal places than the Account currency.")
	case errors.Is(err, app.ErrDateInTheFuture):
		return invalid(InvalidTransactionType, "Invalid Transaction",
			"The date is after today: planned money belongs to Planning.")
	case errors.Is(err, app.ErrBookedBeforeStartDate):
		return invalid(InvalidTransactionType, "Invalid Transaction", "The date is before the Account's start date.")
	case errors.Is(err, app.ErrInternalCategory):
		return invalid(InternalCategoryType, "Internal Category",
			"Internal Categories belong to the system: only Uncategorised can be chosen.")
	case errors.Is(err, app.ErrCategoryNotFound):
		return invalid(InvalidCategoryType, "Invalid Category", "The category_id is not one of the Household's Categories.")
	case err != nil:
		return nil, fmt.Errorf("record cash transaction: %w", err)
	}

	return transactionOut(d)
}
