package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	march          = mar05
	salaryReversal = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000f1")
	// salaryReversed reverses February's salary, so February has no
	// Transactions that aren't reversed.
	salaryReversed = domain.ReversalBooked{
		TransactionID: salaryBooked, ReversalID: salaryReversal, BookingDate: "2026-02-01", Amount: dec("-3000.00"),
	}
	// marchBooked is a Transaction in March.
	marchBooked = domain.TransactionBooked{TransactionID: thirdImport, BookingDate: "2026-03-05", Amount: dec("-1.00")}
)

// twoMonthsClosed is reviewed() with January and February closed: March
// is the Open month, with nothing in it.
func twoMonthsClosed() []domain.AccountEvent {
	return append(reviewed(), januaryClosed, februaryClosed)
}

// reopenRefusal decides a Reopen of month on the Account after past,
// expecting want.
func reopenRefusal(t *testing.T, past []domain.AccountEvent, month string, want *domain.MonthReopenRefusedError) {
	t.Helper()

	m, err := time.Parse(domain.MonthLayout, month)
	if err != nil {
		t.Fatal(err)
	}

	_, err = domain.Account.Decide(decidertest.Given(t, domain.Account, past...).State(), domain.ReopenMonth{Month: m})

	got, ok := errors.AsType[*domain.MonthReopenRefusedError](err)
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("error = %v, want %+v", err, want)
	}

	if !errors.Is(err, domain.ErrMonthReopenRefused) {
		t.Errorf("error = %v, want it to be ErrMonthReopenRefused", err)
	}
}

// The latest closed month is reopened while the Open month has no
// Transactions that aren't reversed: it is the Open month again, its
// Month close is gone, and the month after it isn't open yet (spec stories
// 7, 9; ADR-0024).
func TestReopeningTheLatestClosedMonth(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Account, twoMonthsClosed()...).
		When(domain.ReopenMonth{Month: february}).
		Then(domain.MonthReopened{Month: "2026-02"})

	s := decidertest.Given(t, domain.Account, append(twoMonthsClosed(), domain.MonthReopened{Month: "2026-02"})...).State()

	if s.OpenMonth != "2026-02" {
		t.Errorf("open month = %q, want 2026-02", s.OpenMonth)
	}

	if _, ok := s.Closed["2026-02"]; ok || len(s.Closed) != 1 {
		t.Errorf("closed = %v, want only January", s.Closed)
	}

	assertRefused(t, append(twoMonthsClosed(), domain.MonthReopened{Month: "2026-02"}),
		domain.BookTransaction{TransactionID: marchBooked.TransactionID, BookingDate: march, Amount: dec("-1.00")},
		&domain.MonthError{Err: domain.ErrMonthNotOpenYet, Month: "2026-03", OpenMonth: "2026-02"})
}

// Months reopen one at a time, from the latest back, and a reopened month
// closes again with its Statement balance (spec story 9).
func TestAReopenedMonthClosesAgain(t *testing.T) {
	t.Parallel()

	past := append(twoMonthsClosed(), domain.MonthReopened{Month: "2026-02"}, salaryReversed)

	decidertest.Given(t, domain.Account, past...).
		When(domain.ReopenMonth{Month: january}).
		Then(domain.MonthReopened{Month: "2026-01"})

	past = append(past, domain.MonthReopened{Month: "2026-01"})

	if s := decidertest.Given(t, domain.Account, past...).State(); s.OpenMonth != "2026-01" || len(s.Closed) != 0 {
		t.Errorf("state = %+v, want January open and nothing closed", s)
	}

	decidertest.Given(t, domain.Account, past...).
		When(domain.CloseMonth{Today: closingDay, Month: january, StatementBalance: dec("87.70")}).
		Then(januaryClosed)
}

// A reversal in the Open month takes its Transaction out of the count, so
// the Open month can have none left and the month before it reopens (spec
// story 8).
func TestReopeningAfterAReversalEmptiesTheOpenMonth(t *testing.T) {
	t.Parallel()

	reopenRefusal(t, januaryClosedAccount(), "2026-01", &domain.MonthReopenRefusedError{
		Month: "2026-01", OpenMonth: "2026-02", LatestClosed: "2026-01", Accepted: 1,
	})

	past := append(januaryClosedAccount(), salaryReversed)

	if got := decidertest.Given(t, domain.Account, past...).State().Live["2026-02"]; got != 0 {
		t.Fatalf("live in February = %d, want 0", got)
	}

	decidertest.Given(t, domain.Account, past...).
		When(domain.ReopenMonth{Month: january}).
		Then(domain.MonthReopened{Month: "2026-01"})
}

// Each Reopen condition on its own refuses the Reopen, naming only itself,
// and failed conditions are listed together (spec story 8).
func TestEachReopenConditionRefusesTheReopen(t *testing.T) {
	t.Parallel()

	t.Run("the open month isn't closed", func(t *testing.T) {
		t.Parallel()
		reopenRefusal(t, opened(), "2026-01", &domain.MonthReopenRefusedError{
			Month: "2026-01", OpenMonth: "2026-01", NotClosed: true,
		})
	})

	t.Run("a month not open yet isn't closed", func(t *testing.T) {
		t.Parallel()
		reopenRefusal(t, twoMonthsClosed(), "2026-04", &domain.MonthReopenRefusedError{
			Month: "2026-04", OpenMonth: "2026-03", NotClosed: true, LatestClosed: "2026-02",
		})
	})

	t.Run("a month before the start date isn't closed", func(t *testing.T) {
		t.Parallel()
		reopenRefusal(t, twoMonthsClosed(), "2025-12", &domain.MonthReopenRefusedError{
			Month: "2025-12", OpenMonth: "2026-03", NotClosed: true, LatestClosed: "2026-02",
		})
	})

	t.Run("not the latest closed month", func(t *testing.T) {
		t.Parallel()
		reopenRefusal(t, twoMonthsClosed(), "2026-01", &domain.MonthReopenRefusedError{
			Month: "2026-01", OpenMonth: "2026-03", NotLatestClosed: true, LatestClosed: "2026-02",
		})
	})

	t.Run("the open month has accepted transactions", func(t *testing.T) {
		t.Parallel()
		reopenRefusal(t, append(twoMonthsClosed(), marchBooked), "2026-02", &domain.MonthReopenRefusedError{
			Month: "2026-02", OpenMonth: "2026-03", LatestClosed: "2026-02", Accepted: 1,
		})
	})

	t.Run("not latest and accepted transactions", func(t *testing.T) {
		t.Parallel()
		reopenRefusal(t, append(twoMonthsClosed(), marchBooked), "2026-01", &domain.MonthReopenRefusedError{
			Month: "2026-01", OpenMonth: "2026-03", NotLatestClosed: true, LatestClosed: "2026-02", Accepted: 1,
		})
	})

	t.Run("not closed and accepted transactions", func(t *testing.T) {
		t.Parallel()
		reopenRefusal(t, append(twoMonthsClosed(), marchBooked), "2026-03", &domain.MonthReopenRefusedError{
			Month: "2026-03", OpenMonth: "2026-03", NotClosed: true, LatestClosed: "2026-02", Accepted: 1,
		})
	})
}

// An Account that doesn't exist can't reopen a month.
func TestReopeningAMonthOfAnAccountNotAdded(t *testing.T) {
	t.Parallel()

	decidertest.Given[domain.AccountState](t, domain.Account).
		When(domain.ReopenMonth{Month: january}).
		ThenError(domain.ErrAccountNotAdded)
}
