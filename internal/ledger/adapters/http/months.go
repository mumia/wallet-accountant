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
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// Problem types of Month close.
const (
	MonthCloseRefusedType                   = "/problems/month-close-refused"
	MonthAlreadyClosedType                  = "/problems/month-already-closed"
	AccountAwaitingFirstStatementImportType = "/problems/account-awaiting-first-statement-import"
	InvalidMonthCloseType                   = "/problems/invalid-month-close"
)

// The reasons a Month close is refused, as the API names them.
const (
	reasonPending               = "PENDING"
	reasonSuspectedDuplicates   = "SUSPECTED_DUPLICATES"
	reasonUncategorised         = "UNCATEGORISED"
	reasonBalanceCheckNotPassed = "BALANCE_CHECK_NOT_PASSED"
	reasonBalanceMismatch       = "BALANCE_MISMATCH"
)

// MonthHandler implements the Month operations of oas.Handler.
type MonthHandler struct {
	months *app.Months
}

// NewMonthHandler returns the Month operations of the Ledger HTTP adapter.
func NewMonthHandler(months *app.Months) *MonthHandler {
	return &MonthHandler{months: months}
}

// GetAccountMonth implements getAccountMonth.
func (h *MonthHandler) GetAccountMonth(ctx context.Context, params oas.GetAccountMonthParams) (oas.GetAccountMonthRes, error) {
	month, err := time.Parse(domain.MonthLayout, params.BookingMonth)
	if err != nil {
		return new(oas.GetAccountMonthBadRequest(httpapi.Problem(http.StatusBadRequest, "", "",
			"The booking_month must be written YYYY-MM."))), nil
	}

	got, err := h.months.Status(ctx, params.HouseholdID, params.AccountID, month)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.GetAccountMonthNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("get account month: %w", err)
	}

	return accountMonthOut(got)
}

// CloseAccountMonth implements closeAccountMonth.
func (h *MonthHandler) CloseAccountMonth(ctx context.Context, req *oas.MonthClosing, params oas.CloseAccountMonthParams,
) (oas.CloseAccountMonthRes, error) {
	month, err := time.Parse(domain.MonthLayout, params.BookingMonth)
	if err != nil {
		return new(oas.CloseAccountMonthBadRequest(httpapi.Problem(http.StatusBadRequest, "", "",
			"The booking_month must be written YYYY-MM."))), nil
	}

	balance := req.StatementBalance
	closed, err := h.months.Close(ctx, params.HouseholdID, params.AccountID, month, balance.Amount.Value,
		string(balance.Currency))

	conflict := func(typ, title, detail string) (oas.CloseAccountMonthRes, error) {
		return new(oas.CloseAccountMonthConflict(httpapi.Problem(http.StatusConflict, typ, title, detail))), nil
	}
	invalid := func(detail string) (oas.CloseAccountMonthRes, error) {
		return &oas.MonthCloseProblem{
			Type: InvalidMonthCloseType, Title: "Invalid Month close", Status: http.StatusUnprocessableEntity,
			Detail: oas.NewOptString(detail),
		}, nil
	}

	var refused *domain.MonthCloseRefusedError

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.CloseAccountMonthNotFound(httpapi.NotFoundProblem())), nil
	case errors.As(err, &refused):
		return refusedProblem(refused, req)
	case errors.Is(err, app.ErrMonthAlreadyClosed):
		return conflict(MonthAlreadyClosedType, "Month already closed",
			"The month is already closed; its Statement balance is kept.")
	case errors.Is(err, app.ErrAwaitingFirstStatementImport):
		return conflict(AccountAwaitingFirstStatementImportType, "Account awaiting its first Statement import",
			"The Account has no balance until its first Statement import is confirmed.")
	case errors.Is(err, app.ErrStatementBalanceCurrency):
		return invalid("The Statement balance must be in the Account currency.")
	case errors.Is(err, app.ErrTooManyDecimalPlaces):
		return invalid("The Statement balance has more decimal places than the Account currency.")
	case errors.Is(err, app.ErrMonthBeforeStartDate):
		return invalid("The month is before the Account's start date.")
	case errors.Is(err, app.ErrMonthNotStarted):
		return invalid("The month hasn't started yet, so it has no Statement balance to close with.")
	case err != nil:
		return nil, fmt.Errorf("close account month: %w", err)
	}

	return accountMonthOut(closed)
}

// refusedProblem lists every reason the month can't close, each with its
// details. Amounts are in the Statement balance's currency, which is the
// Account's.
func refusedProblem(e *domain.MonthCloseRefusedError, req *oas.MonthClosing) (*oas.MonthCloseProblem, error) {
	p := &oas.MonthCloseProblem{
		Type: MonthCloseRefusedType, Title: "Month can't close", Status: http.StatusUnprocessableEntity,
		Detail: oas.NewOptString(e.Error()),
	}

	if e.Pending > 0 {
		p.Reasons = append(p.Reasons, reasonPending)
		p.Pending = oas.NewOptInt32(int32(e.Pending)) //nolint:gosec // a count of rows
	}

	if e.SuspectedDuplicates > 0 {
		p.Reasons = append(p.Reasons, reasonSuspectedDuplicates)
		p.SuspectedDuplicates = oas.NewOptInt32(int32(e.SuspectedDuplicates)) //nolint:gosec // a count of rows
	}

	if e.Uncategorised > 0 {
		p.Reasons = append(p.Reasons, reasonUncategorised)
		p.Uncategorised = oas.NewOptInt32(int32(e.Uncategorised)) //nolint:gosec // a count of Transactions
	}

	if len(e.UncheckedImports) > 0 {
		p.Reasons = append(p.Reasons, reasonBalanceCheckNotPassed)

		for _, u := range e.UncheckedImports {
			check := "WAITING"
			if u.Check == domain.ImportCheckFailed {
				check = "FAILED"
			}

			p.UncheckedStatementImports = append(p.UncheckedStatementImports, oas.UncheckedStatementImport{
				StatementImportID: u.StatementImportID, BalanceCheck: check,
			})
		}
	}

	if m := e.Mismatch; m != nil {
		p.Reasons = append(p.Reasons, reasonBalanceMismatch)

		currency := string(req.StatementBalance.Currency)

		for _, field := range []struct {
			to    *oas.OptMoney
			value sharedkernel.Decimal
		}{{&p.ComputedBalance, m.Computed}, {&p.StatementBalance, m.Statement}, {&p.Difference, m.Difference}} {
			out, err := money(field.value, currency)
			if err != nil {
				return nil, err
			}

			*field.to = oas.NewOptMoney(out)
		}
	}

	return p, nil
}

// accountMonthOut is a month's status as the API writes it.
func accountMonthOut(m app.AccountMonth) (*oas.AccountMonth, error) {
	out := &oas.AccountMonth{
		AccountID:           m.AccountID,
		BookingMonth:        m.Month.Format(domain.MonthLayout),
		State:               "OPEN",
		Pending:             int32(m.Pending),             //nolint:gosec // a count of rows
		SuspectedDuplicates: int32(m.SuspectedDuplicates), //nolint:gosec // a count of rows
		Uncategorised:       int32(m.Uncategorised),       //nolint:gosec // a count of Transactions
		Flagged:             m.Flagged,
	}

	if m.ComputedBalance != nil {
		computed, err := money(*m.ComputedBalance, m.Currency)
		if err != nil {
			return nil, err
		}

		out.ComputedBalance = oas.NewOptMoney(computed)
	}

	if m.Closed {
		statement, err := money(m.StatementBalance, m.Currency)
		if err != nil {
			return nil, err
		}

		out.State = "CLOSED"
		out.StatementBalance = oas.NewOptMoney(statement)
		out.ClosedAt = oas.NewOptDateTime(m.ClosedAt)
	}

	return out, nil
}
