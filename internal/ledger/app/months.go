package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var (
	// ErrMonthAlreadyClosed is returned for closing a month twice.
	ErrMonthAlreadyClosed = domain.ErrMonthAlreadyClosed
	// ErrMonthBeforeStartDate is returned for closing a month before the
	// Account's start date.
	ErrMonthBeforeStartDate = domain.ErrMonthBeforeStartDate
	// ErrMonthCloseRefused is returned, as a *domain.MonthCloseRefusedError,
	// for a month that can't close yet.
	ErrMonthCloseRefused = domain.ErrMonthCloseRefused
	// ErrAwaitingFirstStatementImport is returned for closing a month of an
	// Account that has no Opening balance yet.
	ErrAwaitingFirstStatementImport = domain.ErrAwaitingFirstStatementImport
	// ErrTooManyDecimalPlaces is returned for a Statement balance with more
	// decimal places than the Account currency has.
	ErrTooManyDecimalPlaces = domain.ErrTooManyDecimalPlaces
	// ErrStatementBalanceCurrency is returned for a Statement balance in
	// another currency than the Account's.
	ErrStatementBalanceCurrency = errors.New("statement balance isn't in the account currency")
)

// AccountMonth is one Booking month of an Account and what is left to do
// before it closes. ComputedBalance is nil while the Account awaits its
// first Statement import; StatementBalance and ClosedAt are set only once
// the month is closed.
type AccountMonth struct {
	AccountID           uuid.UUID
	Month               time.Time
	Currency            string
	Closed              bool
	StatementBalance    sharedkernel.Decimal
	ClosedAt            time.Time
	ComputedBalance     *sharedkernel.Decimal
	Pending             int
	SuspectedDuplicates int
	Uncategorised       int
	Flagged             bool
}

// Months shows and closes Accounts' Booking months, reading what Ingestion
// leaves unresolved through its port.
type Months struct {
	scope     HouseholdScope
	accounts  AccountAggregate
	ingestion Ingestion
}

// NewMonths returns the Months service.
func NewMonths(scope HouseholdScope, accounts AccountAggregate, ingestion Ingestion) *Months {
	return &Months{scope: scope, accounts: accounts, ingestion: ingestion}
}

// Status returns the Account's Booking month, read from its fold and, for
// its Imported transactions, through Ingestion, in one transaction.
func (m *Months) Status(ctx context.Context, household, account uuid.UUID, month time.Time) (AccountMonth, error) {
	var out AccountMonth

	err := m.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := m.accounts.Folds.Load(ctx, tx, account)
		if err != nil {
			return fmt.Errorf("load account %s: %w", account, err)
		}

		if !found {
			return ErrAccountNotFound
		}

		imports, err := m.ingestion.MonthImports(ctx, tx, account, month)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by Ingestion
		}

		out = accountMonthOf(account, month, fold.State, imports)

		return nil
	})
	if err != nil {
		return AccountMonth{}, fmt.Errorf("month %s of account %s: %w", month.Format(domain.MonthLayout), account, err)
	}

	return out, nil
}

// Close closes the Account's Booking month with the Statement balance, in
// one transaction: Ingestion's pending and Suspected duplicate Imported
// transactions and Balance checks are read through its port, holding the
// Account's Statement imports, and the Account Decider checks every close
// condition, refusing with all that fail as a *domain.MonthCloseRefusedError.
// Nothing is ever booked to make the balances meet (ADR-0018; consistency
// map: Close month).
func (m *Months) Close(ctx context.Context, household, account uuid.UUID, month time.Time,
	statementBalance sharedkernel.Decimal, currency string,
) (AccountMonth, error) {
	var out AccountMonth

	err := m.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := m.accounts.Folds.Load(ctx, tx, account)
		if err != nil {
			return fmt.Errorf("load account %s: %w", account, err)
		}

		switch {
		case !found:
			return ErrAccountNotFound
		case currency != fold.State.Currency:
			return fmt.Errorf("%w: %s, not %s", ErrStatementBalanceCurrency, currency, fold.State.Currency)
		}

		imports, err := m.ingestion.MonthImportsForClose(ctx, tx, account, month)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by Ingestion
		}

		res, err := m.accounts.Execute(ctx, tx, account, domain.CloseMonth{
			Month: month, StatementBalance: statementBalance, Imports: imports,
		})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		out = accountMonthOf(account, month, res.State, imports)

		return nil
	})
	if err != nil {
		return AccountMonth{}, fmt.Errorf("close month %s of account %s: %w", month.Format(domain.MonthLayout), account, err)
	}

	return out, nil
}

func accountMonthOf(account uuid.UUID, month time.Time, s domain.AccountState, imports domain.MonthImports) AccountMonth {
	key := month.Format(domain.MonthLayout)

	out := AccountMonth{
		AccountID:           account,
		Month:               month,
		Currency:            s.Currency,
		Pending:             imports.Pending,
		SuspectedDuplicates: imports.SuspectedDuplicates,
		Uncategorised:       s.Uncategorised[key],
	}

	if s.HasBalance() {
		out.ComputedBalance = new(s.ClosingBalance(key))
	}

	if c, ok := s.Closed[key]; ok {
		out.Closed, out.StatementBalance, out.ClosedAt, out.Flagged = true, c.StatementBalance, c.ClosedAt, c.Flagged
	}

	return out
}
