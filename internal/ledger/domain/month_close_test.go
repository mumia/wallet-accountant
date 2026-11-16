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
	january  = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	february = time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)

	// closingDay is when the tests close their months: every month up to
	// March 2026 has started.
	closingDay = time.Date(2026, time.March, 10, 9, 30, 0, 0, time.UTC)

	laterImport = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000c1")
	thirdImport = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000c2")
)

// reviewed is an Account whose January holds the Opening balance (100.00)
// and a categorised coffee (-12.30), closing at 87.70, and whose February
// holds a categorised salary (3000.00), closing at 3087.70.
func reviewed() []domain.AccountEvent {
	return append(opened(),
		domain.TransactionBooked{TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-12.30")},
		domain.TransactionBooked{TransactionID: salaryBooked, BookingDate: "2026-02-01", Amount: dec("3000.00")},
	)
}

// refusal decides cmd on the Account after past, expecting a
// *MonthCloseRefusedError.
func refusal(t *testing.T, past []domain.AccountEvent, cmd domain.CloseMonth) *domain.MonthCloseRefusedError {
	t.Helper()

	s := decidertest.Given(t, domain.Account, past...).State()

	_, err := domain.Account.Decide(s, cmd)

	var refused *domain.MonthCloseRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want a *MonthCloseRefusedError", err)
	}

	if !errors.Is(err, domain.ErrMonthCloseRefused) {
		t.Errorf("error = %v, want it to be ErrMonthCloseRefused", err)
	}

	return refused
}

// A month whose computed closing balance (the Opening balance plus every
// month up to it) equals the Statement balance, with nothing pending,
// nothing Uncategorised and every relevant Balance check passed, closes
// (spec stories 53, 54).
func TestClosingAMonth(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Account, reviewed()...).
		When(domain.CloseMonth{Today: closingDay, Month: january, StatementBalance: dec("87.70")}).
		Then(domain.MonthClosed{Month: "2026-01", StatementBalance: dec("87.70")})

	decidertest.Given(t, domain.Account, reviewed()...).
		When(domain.CloseMonth{Today: closingDay, Month: february, StatementBalance: dec("3087.7")}).
		Then(domain.MonthClosed{Month: "2026-02", StatementBalance: dec("3087.7")})

	// A month with no Transaction of its own closes at the previous month's
	// balance.
	march := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	decidertest.Given(t, domain.Account, reviewed()...).
		When(domain.CloseMonth{Today: closingDay, Month: march, StatementBalance: dec("3087.70")}).
		Then(domain.MonthClosed{Month: "2026-03", StatementBalance: dec("3087.70")})
}

func TestAClosedMonthIsRecordedInTheFold(t *testing.T) {
	t.Parallel()

	s := decidertest.Given(t, domain.Account, append(reviewed(),
		domain.MonthClosed{Month: "2026-01", StatementBalance: dec("87.70")})...).State()

	c, ok := s.Closed["2026-01"]
	if !ok || !c.StatementBalance.Equal(dec("87.70")) || c.ClosedAt.IsZero() || c.Flagged {
		t.Errorf("closed = %+v, want January closed at 87.70, unflagged", s.Closed)
	}

	if _, ok := s.Closed["2026-02"]; ok {
		t.Errorf("closed = %+v, want February still open", s.Closed)
	}

	if !s.ClosingBalance("2026-01").Equal(dec("87.70")) || !s.ClosingBalance("2026-02").Equal(dec("3087.70")) {
		t.Errorf("closing balances = %s and %s, want 87.70 and 3087.70",
			s.ClosingBalance("2026-01"), s.ClosingBalance("2026-02"))
	}
}

// Each close condition on its own refuses the close (spec stories 55, 56,
// 57, 58a).
func TestEachCloseConditionRefusesTheClose(t *testing.T) {
	t.Parallel()

	t.Run("uncategorised", func(t *testing.T) {
		t.Parallel()

		past := append(opened(),
			domain.TransactionBooked{
				TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-12.30"),
				Uncategorised: true,
			})

		got := refusal(t, past, domain.CloseMonth{Today: closingDay, Month: january, StatementBalance: dec("87.70")})
		want := &domain.MonthCloseRefusedError{Uncategorised: 1}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("refusal = %+v, want %+v", got, want)
		}
	})

	t.Run("balance mismatch", func(t *testing.T) {
		t.Parallel()

		got := refusal(t, reviewed(), domain.CloseMonth{Today: closingDay, Month: january, StatementBalance: dec("90.00")})
		m := got.Mismatch

		if m == nil || !m.Computed.Equal(dec("87.70")) || !m.Statement.Equal(dec("90.00")) || !m.Difference.Equal(dec("2.30")) ||
			got.Uncategorised != 0 || got.Pending != 0 || got.SuspectedDuplicates != 0 || len(got.UncheckedImports) != 0 {
			t.Errorf("refusal = %+v (%+v), want only a mismatch of 87.70 against 90.00, 2.30 apart", got, got.Mismatch)
		}
	})

	t.Run("pending", func(t *testing.T) {
		t.Parallel()

		got := refusal(t, reviewed(), domain.CloseMonth{
			Today: closingDay,
			Month: january, StatementBalance: dec("87.70"), Imports: domain.MonthImports{Pending: 2},
		})

		if !reflect.DeepEqual(got, &domain.MonthCloseRefusedError{Pending: 2}) {
			t.Errorf("refusal = %+v, want two pending", got)
		}
	})

	t.Run("suspected duplicate", func(t *testing.T) {
		t.Parallel()

		got := refusal(t, reviewed(), domain.CloseMonth{
			Today: closingDay,
			Month: january, StatementBalance: dec("87.70"), Imports: domain.MonthImports{SuspectedDuplicates: 1},
		})

		if !reflect.DeepEqual(got, &domain.MonthCloseRefusedError{SuspectedDuplicates: 1}) {
			t.Errorf("refusal = %+v, want one undecided Suspected duplicate", got)
		}
	})

	for _, check := range []domain.ImportCheck{domain.ImportCheckWaiting, domain.ImportCheckFailed} {
		t.Run("balance check "+string(check), func(t *testing.T) {
			t.Parallel()

			unchecked := []domain.UncheckedImport{{StatementImportID: laterImport, Check: check}}
			got := refusal(t, reviewed(), domain.CloseMonth{
				Today: closingDay,
				Month: january, StatementBalance: dec("87.70"), Imports: domain.MonthImports{UncheckedImports: unchecked},
			})

			if !reflect.DeepEqual(got, &domain.MonthCloseRefusedError{UncheckedImports: unchecked}) {
				t.Errorf("refusal = %+v, want the later import's %s check", got, check)
			}
		})
	}
}

// A refused close lists every failed condition at once (spec story 58).
func TestEveryFailedConditionIsListedTogether(t *testing.T) {
	t.Parallel()

	past := append(opened(),
		domain.TransactionBooked{
			TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-12.30"),
			Uncategorised: true,
		},
		domain.TransactionBooked{
			TransactionID: salaryBooked, BookingDate: "2026-01-25", Amount: dec("5.00"),
			Uncategorised: true,
		},
	)
	unchecked := []domain.UncheckedImport{
		{StatementImportID: laterImport, Check: domain.ImportCheckFailed},
		{StatementImportID: thirdImport, Check: domain.ImportCheckWaiting},
	}

	got := refusal(t, past, domain.CloseMonth{
		Today: closingDay,
		Month: january, StatementBalance: dec("80.00"),
		Imports: domain.MonthImports{Pending: 3, SuspectedDuplicates: 1, UncheckedImports: unchecked},
	})

	if got.Uncategorised != 2 || got.Pending != 3 || got.SuspectedDuplicates != 1 ||
		!reflect.DeepEqual(got.UncheckedImports, unchecked) || got.Mismatch == nil ||
		!got.Mismatch.Computed.Equal(dec("92.70")) || !got.Mismatch.Statement.Equal(dec("80.00")) ||
		!got.Mismatch.Difference.Equal(dec("-12.70")) {
		t.Errorf("refusal = %+v (%+v), want every condition", got, got.Mismatch)
	}
}

func TestClosingAMonthRefusals(t *testing.T) {
	t.Parallel()

	closeJanuary := domain.CloseMonth{Today: closingDay, Month: january, StatementBalance: dec("87.70")}

	decidertest.Given(t, domain.Account).When(closeJanuary).ThenError(domain.ErrAccountNotAdded)
	decidertest.Given(t, domain.Account, mainAdded).When(closeJanuary).ThenError(domain.ErrAwaitingFirstStatementImport)

	// mainAdded starts on 2026-01-15: December is before its history.
	december := domain.CloseMonth{Today: closingDay, Month: time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC), StatementBalance: dec("0")}
	decidertest.Given(t, domain.Account, reviewed()...).When(december).ThenError(domain.ErrMonthBeforeStartDate)

	// A month that hasn't started yet on the day of the close can't close;
	// the month in progress can.
	april := domain.CloseMonth{
		Today: closingDay, Month: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC), StatementBalance: dec("3087.70"),
	}
	decidertest.Given(t, domain.Account, reviewed()...).When(april).ThenError(domain.ErrMonthNotStarted)

	tooPrecise := closeJanuary
	tooPrecise.StatementBalance = dec("87.701")
	decidertest.Given(t, domain.Account, reviewed()...).When(tooPrecise).ThenError(domain.ErrTooManyDecimalPlaces)

	// Closing an already closed month is refused, so its Statement balance
	// isn't overwritten by accident (spec story 64).
	decidertest.Given(t, domain.Account, append(reviewed(),
		domain.MonthClosed{Month: "2026-01", StatementBalance: dec("87.70")})...).
		When(closeJanuary).
		ThenError(domain.ErrMonthAlreadyClosed)

	// Another month still closes.
	decidertest.Given(t, domain.Account, append(reviewed(),
		domain.MonthClosed{Month: "2026-01", StatementBalance: dec("87.70")})...).
		When(domain.CloseMonth{Today: closingDay, Month: february, StatementBalance: dec("3087.70")}).
		Then(domain.MonthClosed{Month: "2026-02", StatementBalance: dec("3087.70")})
}
