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
	// ErrMonthAlreadyClosed is returned, as a *domain.MonthError, for
	// closing a month before the Open month.
	ErrMonthAlreadyClosed = domain.ErrMonthAlreadyClosed
	// ErrMonthNotOpenYet is returned, as a *domain.MonthError, for closing
	// a month after the Open month.
	ErrMonthNotOpenYet = domain.ErrMonthNotOpenYet
	// ErrMonthNotEnded is returned, as a *domain.MonthError, for closing
	// the Open month before it has ended.
	ErrMonthNotEnded = domain.ErrMonthNotEnded
	// ErrMonthClosed is returned, as a *domain.MonthError, for booking or
	// reversing anything in a closed month.
	ErrMonthClosed = domain.ErrMonthClosed
	// ErrUncategorisedInClosedMonth is returned, as a *domain.MonthError,
	// for moving a Transaction of a closed month to Uncategorised.
	ErrUncategorisedInClosedMonth = domain.ErrUncategorisedInClosedMonth
	// ErrMonthBeforeStartDate is returned for closing a month before the
	// Account's start date.
	ErrMonthBeforeStartDate = domain.ErrMonthBeforeStartDate
	// ErrMonthCloseRefused is returned, as a *domain.MonthCloseRefusedError,
	// for a month that can't close yet.
	ErrMonthCloseRefused = domain.ErrMonthCloseRefused
	// ErrMonthReopenRefused is returned, as a
	// *domain.MonthReopenRefusedError, for a month that can't be reopened.
	ErrMonthReopenRefused = domain.ErrMonthReopenRefused
	// ErrAwaitingFirstStatementImport is returned for closing a month of an
	// Account that has no Opening balance yet.
	ErrAwaitingFirstStatementImport = domain.ErrAwaitingFirstStatementImport
	// ErrTooManyDecimalPlaces is returned for a Statement balance with more
	// decimal places than the Account currency has.
	ErrTooManyDecimalPlaces = domain.ErrTooManyDecimalPlaces
	// ErrCountedOnRequired is returned for closing a cash Account's month
	// without the day its cash was counted.
	ErrCountedOnRequired = domain.ErrCountedOnRequired
	// ErrCountedOnNotForStatementAccount is returned for a Cash count's day
	// sent to close a statement Account's month.
	ErrCountedOnNotForStatementAccount = domain.ErrCountedOnNotForStatementAccount
	// ErrInvalidCountDate is returned for a Cash count taken on or before
	// the month's last day, or after today.
	ErrInvalidCountDate = domain.ErrInvalidCountDate
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
	// State is the month's state as the API writes it: OPEN, CLOSED or
	// NOT_OPEN_YET (ADR-0024).
	State string
	// UncheckedImports are the Statement imports with rows in or before
	// the month whose Balance check hasn't passed.
	UncheckedImports []domain.UncheckedImport
	// ReopenBlocked says a Reopen of the month would be refused now.
	ReopenBlocked bool
	// CountedOn is the day a closed cash month's Cash count was taken; zero
	// otherwise.
	CountedOn time.Time
	// LegsInTransit counts the month's Transfer legs that are neither
	// Matched nor reversed; they never stop a close.
	LegsInTransit int
}

// The month states (ADR-0024).
const (
	MonthOpen       = "OPEN"
	MonthClosed     = "CLOSED"
	MonthNotOpenYet = "NOT_OPEN_YET"
)

// MonthReader reads, from the Ledger's lookup rows, what an Account's
// months show beyond its fold.
type MonthReader interface {
	// LegsInTransit counts the Account's Transfer legs that are neither
	// Matched nor reversed, per Booking month (MonthLayout); a month with
	// none is absent.
	LegsInTransit(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (map[string]int, error)
	// MovedAfter locks the Account's fold row for update, then returns how
	// much its Postings booked after the day after and up to and including
	// the day through moved its balance. The lock keeps a Transaction from
	// being booked in that window before the close is saved.
	MovedAfter(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, after, through time.Time,
	) (sharedkernel.Decimal, error)
}

// Months shows and closes Accounts' Booking months, reading what Ingestion
// leaves unresolved through its port.
type Months struct {
	scope     HouseholdScope
	accounts  AccountAggregate
	ingestion Ingestion
	reader    MonthReader
}

// NewMonths returns the Months service.
func NewMonths(scope HouseholdScope, accounts AccountAggregate, ingestion Ingestion, reader MonthReader) *Months {
	return &Months{scope: scope, accounts: accounts, ingestion: ingestion, reader: reader}
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

		switch {
		case !found:
			return ErrAccountNotFound
		case month.Format(domain.MonthLayout) < fold.State.StartDate.Format(domain.MonthLayout):
			return ErrMonthBeforeStartDate
		}

		imports, err := m.ingestion.MonthImports(ctx, tx, account, month)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by Ingestion
		}

		inTransit, err := m.reader.LegsInTransit(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the adapter
		}

		out = accountMonthOf(account, month, fold.State, imports, inTransit)

		return nil
	})
	if err != nil {
		return AccountMonth{}, fmt.Errorf("month %s of account %s: %w", month.Format(domain.MonthLayout), account, err)
	}

	return out, nil
}

// List returns every Booking month of the Account, newest first: from its
// start date's month to the latest of the current calendar month (UTC, by
// the server's clock) and the latest Booking month holding a non-Discarded
// Imported transaction. It reads the Account's fold and, through
// Ingestion, every month's counts and unchecked imports at once, in one
// transaction.
func (m *Months) List(ctx context.Context, household, account uuid.UUID) ([]AccountMonth, error) {
	var out []AccountMonth

	err := m.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := m.accounts.Folds.Load(ctx, tx, account)
		if err != nil {
			return fmt.Errorf("load account %s: %w", account, err)
		}

		if !found {
			return ErrAccountNotFound
		}

		imports, err := m.ingestion.AccountImports(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by Ingestion
		}

		inTransit, err := m.reader.LegsInTransit(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the adapter
		}

		first := fold.State.StartDate.Format(domain.MonthLayout)
		last := max(first, tx.Now().UTC().Format(domain.MonthLayout), imports.LatestMonth())

		for key := last; key >= first; key = domain.PreviousMonth(key) {
			// Every key is written with MonthLayout.
			month, _ := time.Parse(domain.MonthLayout, key)
			out = append(out, accountMonthOf(account, month, fold.State, imports.Month(key), inTransit))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("months of account %s: %w", account, err)
	}

	return out, nil
}

// Close closes the Account's Open month with the Statement balance, once
// it has ended by the server's clock in UTC, and opens the month after it
// (ADR-0024). A cash Account's month closes by a Cash count instead:
// statementBalance is the amount counted on the day countedOn, compared with
// the balance at the end of that day, which adds the Account Posting lookup
// rows booked after the month to its closing balance (consistency map:
// Close month (cash)). It runs in one transaction: Ingestion's pending and Suspected duplicate Imported
// transactions and Balance checks are read through its port, holding the
// Account's Statement imports, and the Account Decider checks every close
// condition, refusing with all that fail as a *domain.MonthCloseRefusedError.
// Nothing is ever booked to make the balances meet (ADR-0018; consistency
// map: Close month).
func (m *Months) Close(ctx context.Context, household, account uuid.UUID, month time.Time,
	statementBalance sharedkernel.Decimal, currency string, countedOn *time.Time,
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

		var moved sharedkernel.Decimal

		if countedOn != nil && fold.State.IsCash() {
			monthEnd := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, time.UTC)

			if moved, err = m.reader.MovedAfter(ctx, tx, account, monthEnd, *countedOn); err != nil {
				return err //nolint:wrapcheck // already wrapped by the adapter
			}
		}

		res, err := m.accounts.Execute(ctx, tx, account, domain.CloseMonth{
			Today: tx.Now(), Month: month, StatementBalance: statementBalance, Imports: imports,
			CountedOn: countedOn, MovedByCountedOn: moved,
		})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		inTransit, err := m.reader.LegsInTransit(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the adapter
		}

		out = accountMonthOf(account, month, res.State, imports, inTransit)

		return nil
	})
	if err != nil {
		return AccountMonth{}, fmt.Errorf("close month %s of account %s: %w", month.Format(domain.MonthLayout), account, err)
	}

	return out, nil
}

// Reopen takes back the Account's latest Month close while its Open month
// has no Accepted Transactions that aren't reversed: the month is the Open
// month again, and the month after it is not open yet (ADR-0024). A cash
// Account skips that condition. The Account Decider refuses with every
// failed condition together, as a *domain.MonthReopenRefusedError
// (consistency map: Reopen month, Reopen month (cash)).
func (m *Months) Reopen(ctx context.Context, household, account uuid.UUID, month time.Time) (AccountMonth, error) {
	var out AccountMonth

	err := m.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		_, found, err := m.accounts.Folds.Load(ctx, tx, account)
		if err != nil {
			return fmt.Errorf("load account %s: %w", account, err)
		}

		if !found {
			return ErrAccountNotFound
		}

		res, err := m.accounts.Execute(ctx, tx, account, domain.ReopenMonth{Month: month})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		imports, err := m.ingestion.MonthImports(ctx, tx, account, month)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by Ingestion
		}

		inTransit, err := m.reader.LegsInTransit(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the adapter
		}

		out = accountMonthOf(account, month, res.State, imports, inTransit)

		return nil
	})
	if err != nil {
		return AccountMonth{}, fmt.Errorf("reopen month %s of account %s: %w", month.Format(domain.MonthLayout), account, err)
	}

	return out, nil
}

func accountMonthOf(account uuid.UUID, month time.Time, s domain.AccountState, imports domain.MonthImports,
	inTransit map[string]int,
) AccountMonth {
	key := month.Format(domain.MonthLayout)

	out := AccountMonth{
		AccountID:           account,
		Month:               month,
		Currency:            s.Currency,
		Pending:             imports.Pending,
		SuspectedDuplicates: imports.SuspectedDuplicates,
		Uncategorised:       s.Uncategorised[key],
		UncheckedImports:    imports.UncheckedImports,
		ReopenBlocked:       s.ReopenRefusal(key) != nil,
		LegsInTransit:       inTransit[key],
	}

	switch {
	case key < s.OpenMonth:
		out.State = MonthClosed
	case key == s.OpenMonth:
		out.State = MonthOpen
	default:
		out.State = MonthNotOpenYet
	}

	if s.HasBalance() {
		out.ComputedBalance = new(s.ClosingBalance(key))
	}

	if c, ok := s.Closed[key]; ok {
		out.Closed, out.StatementBalance, out.ClosedAt, out.CountedOn = true, c.StatementBalance, c.ClosedAt, c.CountedOn
	}

	return out
}
