package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
	MonthNotEndedType                       = "/problems/month-not-ended"
	AccountAwaitingFirstStatementImportType = "/problems/account-awaiting-first-statement-import"
	InvalidMonthCloseType                   = "/problems/invalid-month-close"
	MonthReopenRefusedType                  = "/problems/month-reopen-refused"
)

// The reasons a Month close is refused, as the API names them.
const (
	reasonPending               = "PENDING"
	reasonSuspectedDuplicates   = "SUSPECTED_DUPLICATES"
	reasonUncategorised         = "UNCATEGORISED"
	reasonBalanceCheckNotPassed = "BALANCE_CHECK_NOT_PASSED"
	reasonBalanceMismatch       = "BALANCE_MISMATCH"
)

// The reasons a Reopen is refused, as the API names them.
const (
	reasonNotClosed                        = "NOT_CLOSED"
	reasonNotLatestClosed                  = "NOT_LATEST_CLOSED"
	reasonOpenMonthHasAcceptedTransactions = "OPEN_MONTH_HAS_ACCEPTED_TRANSACTIONS"
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
		return httpapi.MonthConflict(typ, title, detail, "", ""), nil
	}
	monthConflict := func(typ, title, detail string, e *domain.MonthError) (oas.CloseAccountMonthRes, error) {
		return httpapi.MonthConflict(typ, title, detail, e.Month, e.OpenMonth), nil
	}
	invalid := func(detail string) (oas.CloseAccountMonthRes, error) {
		return &oas.MonthCloseProblem{
			Type: InvalidMonthCloseType, Title: "Invalid Month close", Status: http.StatusUnprocessableEntity,
			Detail: oas.NewOptString(detail),
		}, nil
	}

	var (
		refused  *domain.MonthCloseRefusedError
		monthErr *domain.MonthError
	)

	errors.As(err, &monthErr)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.CloseAccountMonthNotFound(httpapi.NotFoundProblem())), nil
	case errors.As(err, &refused):
		return refusedProblem(refused, req)
	case errors.Is(err, app.ErrMonthAlreadyClosed) && monthErr != nil:
		return monthConflict(MonthAlreadyClosedType, "Month already closed",
			httpapi.MonthWords(monthErr.Month)+" is already closed; its Statement balance is kept.", monthErr)
	case errors.Is(err, app.ErrMonthNotOpenYet) && monthErr != nil:
		return httpapi.MonthNotOpenYet(monthErr.Month, monthErr.OpenMonth), nil
	case errors.Is(err, app.ErrMonthNotEnded) && monthErr != nil:
		return monthConflict(MonthNotEndedType, "Month not ended",
			fmt.Sprintf("%s can be closed from %s.",
				httpapi.MonthWords(monthErr.Month), httpapi.DateWords(month.AddDate(0, 1, 0))), monthErr)
	case errors.Is(err, app.ErrAwaitingFirstStatementImport):
		return conflict(AccountAwaitingFirstStatementImportType, "Account awaiting its first Statement import",
			"The Account has no balance until its first Statement import is confirmed.")
	case errors.Is(err, app.ErrStatementBalanceCurrency):
		return invalid("The Statement balance must be in the Account currency.")
	case errors.Is(err, app.ErrTooManyDecimalPlaces):
		return invalid("The Statement balance has more decimal places than the Account currency.")
	case errors.Is(err, app.ErrMonthBeforeStartDate):
		return invalid("The month is before the Account's start date.")
	case err != nil:
		return nil, fmt.Errorf("close account month: %w", err)
	}

	return accountMonthOut(closed)
}

// ReopenAccountMonth implements reopenAccountMonth.
func (h *MonthHandler) ReopenAccountMonth(ctx context.Context, params oas.ReopenAccountMonthParams,
) (oas.ReopenAccountMonthRes, error) {
	month, err := time.Parse(domain.MonthLayout, params.BookingMonth)
	if err != nil {
		return new(oas.ReopenAccountMonthBadRequest(httpapi.Problem(http.StatusBadRequest, "", "",
			"The booking_month must be written YYYY-MM."))), nil
	}

	reopened, err := h.months.Reopen(ctx, params.HouseholdID, params.AccountID, month)

	var refused *domain.MonthReopenRefusedError

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.ReopenAccountMonthNotFound(httpapi.NotFoundProblem())), nil
	case errors.As(err, &refused):
		return reopenRefusedProblem(refused), nil
	case err != nil:
		return nil, fmt.Errorf("reopen account month: %w", err)
	}

	return accountMonthOut(reopened)
}

// reopenRefusedProblem lists every reason the month can't be reopened,
// with one sentence per reason in detail.
func reopenRefusedProblem(e *domain.MonthReopenRefusedError) *oas.MonthReopenProblem {
	month := httpapi.MonthWords(e.Month)

	p := &oas.MonthReopenProblem{
		Type: MonthReopenRefusedType, Title: "Month can't be reopened", Status: http.StatusConflict,
		BookingMonth: oas.NewOptString(e.Month), OpenMonth: oas.NewOptString(e.OpenMonth),
	}

	var sentences []string

	if e.NotClosed {
		p.Reasons = append(p.Reasons, reasonNotClosed)
		sentences = append(sentences, month+" isn't closed, so it can't be reopened.")
	}

	if e.NotLatestClosed {
		p.Reasons = append(p.Reasons, reasonNotLatestClosed)
		sentences = append(sentences, fmt.Sprintf("Only %s, the latest closed month, can be reopened.",
			httpapi.MonthWords(e.LatestClosed)))
	}

	if e.Accepted > 0 {
		p.Reasons = append(p.Reasons, reasonOpenMonthHasAcceptedTransactions)
		p.Accepted = oas.NewOptInt32(int32(e.Accepted)) //nolint:gosec // a count of Transactions

		transactions, them := "Accepted Transactions", "them"
		if e.Accepted == 1 {
			transactions, them = "Accepted Transaction", "it"
		}

		sentences = append(sentences, fmt.Sprintf("%s has %d %s. Reverse %s before reopening %s.",
			httpapi.MonthWords(e.OpenMonth), e.Accepted, transactions, them, month))
	}

	p.Detail = oas.NewOptString(strings.Join(sentences, " "))

	return p
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
			p.UncheckedStatementImports = append(p.UncheckedStatementImports, uncheckedImportOut(u))
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
		State:               m.State,
		Pending:             int32(m.Pending),             //nolint:gosec // a count of rows
		SuspectedDuplicates: int32(m.SuspectedDuplicates), //nolint:gosec // a count of rows
		Uncategorised:       int32(m.Uncategorised),       //nolint:gosec // a count of Transactions
		ReopenBlocked:       m.ReopenBlocked,
	}

	out.UncheckedStatementImports = make([]oas.UncheckedStatementImport, 0, len(m.UncheckedImports))
	for _, u := range m.UncheckedImports {
		out.UncheckedStatementImports = append(out.UncheckedStatementImports, uncheckedImportOut(u))
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

		out.StatementBalance = oas.NewOptMoney(statement)
		out.ClosedAt = oas.NewOptDateTime(m.ClosedAt)
	}

	return out, nil
}

// uncheckedImportOut is a Statement import whose Balance check stops a
// Month close, as the API writes it.
func uncheckedImportOut(u domain.UncheckedImport) oas.UncheckedStatementImport {
	check := "WAITING"
	if u.Check == domain.ImportCheckFailed {
		check = "FAILED"
	}

	return oas.UncheckedStatementImport{StatementImportID: u.StatementImportID, BalanceCheck: check}
}
