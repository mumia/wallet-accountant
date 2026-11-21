package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var (
	// ErrMonthAlreadyClosed is returned for closing a month a second time:
	// its Statement balance isn't overwritten (spec story 64).
	ErrMonthAlreadyClosed = errors.New("month already closed")
	// ErrMonthBeforeStartDate is returned for closing a month before the
	// Account's start date, where its history doesn't reach.
	ErrMonthBeforeStartDate = errors.New("month is before the account's start date")
	// ErrMonthNotStarted is returned for closing a month later than the
	// month the close is given in: it has no Statement yet.
	ErrMonthNotStarted = errors.New("month hasn't started yet")
	// ErrMonthCloseRefused is returned, as a *MonthCloseRefusedError, for a
	// month that can't close yet.
	ErrMonthCloseRefused = errors.New("month close refused")
)

// ImportCheck is the state of a Statement import's Balance check that
// stops a Month close.
type ImportCheck string

// The Balance check states that stop a Month close (ADR-0018).
const (
	ImportCheckWaiting ImportCheck = "waiting"
	ImportCheckFailed  ImportCheck = "failed"
)

// UncheckedImport is a Statement import with rows in or before a month
// whose Balance check is waiting or failed.
type UncheckedImport struct {
	StatementImportID sharedkernel.ID
	Check             ImportCheck
}

// MonthImports is what Ingestion reports about an Account's Booking month:
// its unresolved Imported transactions, and the Statement imports with rows
// in or before it whose Balance check hasn't passed. Voided imports never
// count.
type MonthImports struct {
	Pending             int
	SuspectedDuplicates int
	UncheckedImports    []UncheckedImport
}

// BalanceMismatch is a computed closing balance that differs from the
// Statement balance; Difference is the Statement balance minus the
// computed one.
type BalanceMismatch struct {
	Computed   sharedkernel.Decimal
	Statement  sharedkernel.Decimal
	Difference sharedkernel.Decimal
}

// MonthCloseRefusedError lists every reason a month can't close (spec
// story 58). A zero count or a nil Mismatch is a condition that holds.
type MonthCloseRefusedError struct {
	Uncategorised       int
	Pending             int
	SuspectedDuplicates int
	UncheckedImports    []UncheckedImport
	Mismatch            *BalanceMismatch
}

func (e *MonthCloseRefusedError) Error() string {
	var reasons []string

	if e.Pending > 0 {
		reasons = append(reasons, fmt.Sprintf("%d pending", e.Pending))
	}

	if e.SuspectedDuplicates > 0 {
		reasons = append(reasons, fmt.Sprintf("%d undecided suspected duplicates", e.SuspectedDuplicates))
	}

	if e.Uncategorised > 0 {
		reasons = append(reasons, fmt.Sprintf("%d uncategorised", e.Uncategorised))
	}

	for _, u := range e.UncheckedImports {
		reasons = append(reasons, fmt.Sprintf("statement import %s balance check %s", u.StatementImportID, u.Check))
	}

	if m := e.Mismatch; m != nil {
		reasons = append(reasons, fmt.Sprintf("computed balance %s, statement balance %s, difference %s",
			m.Computed, m.Statement, m.Difference))
	}

	return fmt.Sprintf("%s: %s", ErrMonthCloseRefused, strings.Join(reasons, "; "))
}

func (e *MonthCloseRefusedError) Unwrap() error { return ErrMonthCloseRefused }

// MonthClose is a closed Booking month: the Statement balance it closed
// with, when and by whom, and whether it changed after it closed.
type MonthClose struct {
	StatementBalance sharedkernel.Decimal
	ClosedAt         time.Time
	ClosedBy         sharedkernel.ID
	Flagged          bool
}

// CloseMonth closes the Booking month Month (any day in it) with the
// Statement balance, on the day Today. Imports is what Ingestion reported
// about the month.
type CloseMonth struct {
	Today            time.Time
	Month            time.Time
	StatementBalance sharedkernel.Decimal
	Imports          MonthImports
}

func (CloseMonth) isAccountCommand() {}

// MonthClosed is recorded when a Booking month (MonthLayout) closes with
// its Statement balance. When and by whom are the event's own.
type MonthClosed struct {
	Month            string               `json:"month"`
	StatementBalance sharedkernel.Decimal `json:"statementbalance"`
}

func (MonthClosed) isAccountEvent() {}

// OpenMonth is the Account's one open Booking month (MonthLayout): the
// first month from the start date's month that isn't closed (ADR-0024).
// Until the fold holds it (ticket 02 of .scratch/ui-first-slice-backend),
// it is derived from the Month closes.
func (s AccountState) OpenMonth() string {
	return FirstOpenMonth(s.StartDate, func(month string) bool {
		_, closed := s.Closed[month]

		return closed
	})
}

// FirstOpenMonth is the first Booking month (MonthLayout) from the start
// date's month that closed doesn't report as closed.
func FirstOpenMonth(startDate time.Time, closed func(month string) bool) string {
	month := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.UTC)

	for {
		key := month.Format(MonthLayout)
		if !closed(key) {
			return key
		}

		month = month.AddDate(0, 1, 0)
	}
}

// ClosingBalance is the computed balance at the end of the Booking month
// (MonthLayout): the Opening balance plus every month up to this one.
func (s AccountState) ClosingBalance(month string) sharedkernel.Decimal {
	var sum sharedkernel.Decimal

	for m, change := range s.Months {
		if m <= month {
			sum = sum.Add(change)
		}
	}

	return sum
}

// closeMonth checks every close condition and refuses with all of those
// that fail together. There is never a balancing entry (ADR-0018).
func closeMonth(s AccountState, c CloseMonth) ([]AccountEvent, error) {
	month := c.Month.Format(MonthLayout)

	switch {
	case !s.Added:
		return nil, ErrAccountNotAdded
	case s.AwaitingFirstStatementImport:
		return nil, ErrAwaitingFirstStatementImport
	case month < s.StartDate.Format(MonthLayout):
		return nil, fmt.Errorf("%w: %s is before %s", ErrMonthBeforeStartDate, month, s.StartDate.Format(DateLayout))
	case month > c.Today.Format(MonthLayout):
		return nil, fmt.Errorf("%w: %s is after %s", ErrMonthNotStarted, month, c.Today.Format(DateLayout))
	}

	if _, closed := s.Closed[month]; closed {
		return nil, fmt.Errorf("%w: %s", ErrMonthAlreadyClosed, month)
	}

	currency, err := sharedkernel.ParseCurrency(s.Currency)
	if err != nil {
		return nil, err //nolint:wrapcheck // the shared kernel's own refusal
	}

	if !c.StatementBalance.Equal(c.StatementBalance.Round(int32(currency.Decimals()))) { //nolint:gosec // at most 18 places
		return nil, fmt.Errorf("%w: %s %s", ErrTooManyDecimalPlaces, c.StatementBalance, currency.Code())
	}

	refused := &MonthCloseRefusedError{
		Uncategorised:       s.Uncategorised[month],
		Pending:             c.Imports.Pending,
		SuspectedDuplicates: c.Imports.SuspectedDuplicates,
		UncheckedImports:    c.Imports.UncheckedImports,
	}

	if computed := s.ClosingBalance(month); !computed.Equal(c.StatementBalance) {
		refused.Mismatch = &BalanceMismatch{
			Computed: computed, Statement: c.StatementBalance, Difference: c.StatementBalance.Sub(computed),
		}
	}

	if refused.Uncategorised > 0 || refused.Pending > 0 || refused.SuspectedDuplicates > 0 ||
		len(refused.UncheckedImports) > 0 || refused.Mismatch != nil {
		return nil, refused
	}

	return []AccountEvent{MonthClosed{Month: month, StatementBalance: c.StatementBalance}}, nil
}

// closed returns a copy of closes with month closed, so a fold never
// shares its map with an earlier state.
func closed(closes map[string]MonthClose, month string, c MonthClose) map[string]MonthClose {
	out := make(map[string]MonthClose, len(closes)+1)
	for m, v := range closes {
		out[m] = v
	}

	out[month] = c

	return out
}

// flagged returns closes with month flagged when it is closed: a change
// after close is allowed but flagged (spec story 62). An open month is left
// out, and closes is returned as is, so a fold never shares a changed map
// with an earlier state.
func flagged(closes map[string]MonthClose, month string) map[string]MonthClose {
	c, ok := closes[month]
	if !ok || c.Flagged {
		return closes
	}

	c.Flagged = true

	return closed(closes, month, c)
}

// flaggedFrom returns closes with every closed month from month on flagged:
// a change that moves the balance of month moves the closing balance of
// every later month too (ClosingBalance), so each of them is touched.
func flaggedFrom(closes map[string]MonthClose, month string) map[string]MonthClose {
	for m := range closes {
		if m >= month {
			closes = flagged(closes, m)
		}
	}

	return closes
}

// FlagMonth flags the closed Booking month of Month (any day in it) for a
// change that touched it without booking anything on the Account: voiding
// a Statement import whose rows were in it (spec story 62).
type FlagMonth struct {
	Month time.Time
}

func (FlagMonth) isAccountCommand() {}

// MonthFlagged is recorded when a closed Booking month (MonthLayout) is
// flagged by a change that didn't book anything on the Account.
type MonthFlagged struct {
	Month string `json:"month"`
}

func (MonthFlagged) isAccountEvent() {}

// flagMonth flags a closed month once; an open or already flagged month
// records nothing.
func flagMonth(s AccountState, c FlagMonth) ([]AccountEvent, error) {
	if !s.Added {
		return nil, ErrAccountNotAdded
	}

	month := c.Month.Format(MonthLayout)
	if m, ok := s.Closed[month]; !ok || m.Flagged {
		return nil, nil
	}

	return []AccountEvent{MonthFlagged{Month: month}}, nil
}
