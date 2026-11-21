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
	feb10       = time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC)
	mar05       = time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC)
	newReversal = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000e1")
)

// januaryClosedAccount is reviewed() with January closed: February is the
// Open month and March isn't open yet.
func januaryClosedAccount() []domain.AccountEvent {
	return append(reviewed(), januaryClosed)
}

// assertRefused decides cmd on the Account after past, expecting want.
func assertRefused(t *testing.T, past []domain.AccountEvent, cmd domain.AccountCommand, want *domain.MonthError) {
	t.Helper()

	_, err := domain.Account.Decide(decidertest.Given(t, domain.Account, past...).State(), cmd)
	if got, ok := errors.AsType[*domain.MonthError](err); !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("error = %v, want %v", err, want)
	}
}

// A Transaction is booked only in the Open month: a closed month is
// locked, and a later month isn't open yet (spec stories 5, 10).
func TestBookingOnlyInTheOpenMonth(t *testing.T) {
	t.Parallel()

	book := func(date time.Time) domain.BookTransaction {
		return domain.BookTransaction{TransactionID: thirdImport, BookingDate: date, Amount: dec("-1.00")}
	}

	assertRefused(t, januaryClosedAccount(), book(jan20),
		&domain.MonthError{Err: domain.ErrMonthClosed, Month: "2026-01", OpenMonth: "2026-02"})
	assertRefused(t, januaryClosedAccount(), book(mar05),
		&domain.MonthError{Err: domain.ErrMonthNotOpenYet, Month: "2026-03", OpenMonth: "2026-02"})
	assertRefused(t, reviewed(), book(feb10),
		&domain.MonthError{Err: domain.ErrMonthNotOpenYet, Month: "2026-02", OpenMonth: "2026-01"})

	decidertest.Given(t, domain.Account, januaryClosedAccount()...).
		When(book(feb10)).
		Then(domain.TransactionBooked{TransactionID: thirdImport, BookingDate: "2026-02-10", Amount: dec("-1.00")})
}

// A Transaction of a closed month can't be reversed; one of the Open
// month, or of a month not open yet, can (spec story 5).
func TestReversingInAClosedMonthIsRefused(t *testing.T) {
	t.Parallel()

	reverse := func(id sharedkernel.ID, date time.Time, amount string) domain.BookReversal {
		return domain.BookReversal{TransactionID: id, ReversalID: newReversal, BookingDate: date, Amount: dec(amount)}
	}

	assertRefused(t, januaryClosedAccount(), reverse(coffeeBooked, jan20, "12.30"),
		&domain.MonthError{Err: domain.ErrMonthClosed, Month: "2026-01", OpenMonth: "2026-02"})

	salaryDay := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	decidertest.Given(t, domain.Account, januaryClosedAccount()...).
		When(reverse(salaryBooked, salaryDay, "-3000.00")).
		Then(domain.ReversalBooked{
			TransactionID: salaryBooked, ReversalID: newReversal, BookingDate: "2026-02-01", Amount: dec("-3000.00"),
		})

	decidertest.Given(t, domain.Account, reviewed()...).
		When(reverse(salaryBooked, salaryDay, "-3000.00")).
		Then(domain.ReversalBooked{
			TransactionID: salaryBooked, ReversalID: newReversal, BookingDate: "2026-02-01", Amount: dec("-3000.00"),
		})
}

// In a closed month a Category can still change, but never to
// Uncategorised (spec story 6).
func TestRecategorisingInAClosedMonth(t *testing.T) {
	t.Parallel()

	recategorise := func(date time.Time, was, now bool) domain.RecategoriseTransaction {
		return domain.RecategoriseTransaction{
			TransactionID: coffeeBooked, BookingDate: date, WasUncategorised: was, Uncategorised: now,
		}
	}

	assertRefused(t, januaryClosedAccount(), recategorise(jan20, false, true),
		&domain.MonthError{Err: domain.ErrUncategorisedInClosedMonth, Month: "2026-01", OpenMonth: "2026-02"})

	decidertest.Given(t, domain.Account, januaryClosedAccount()...).
		When(recategorise(jan20, false, false)).
		Then(domain.TransactionRecategorised{TransactionID: coffeeBooked, BookingDate: "2026-01-20"})

	decidertest.Given(t, domain.Account, januaryClosedAccount()...).
		When(recategorise(feb10, false, true)).
		Then(domain.TransactionRecategorised{TransactionID: coffeeBooked, BookingDate: "2026-02-10", Uncategorised: true})
}

// The Opening balance can't be taken back once the start date's month is
// closed.
func TestReopeningTheOpeningBalanceOfAClosedStartMonthIsRefused(t *testing.T) {
	t.Parallel()

	reopen := domain.ReopenOpeningBalance{TransactionID: openingTransaction, ReversalID: newReversal, Amount: dec("-100.00")}

	assertRefused(t, januaryClosedAccount(), reopen,
		&domain.MonthError{Err: domain.ErrMonthClosed, Month: "2026-01", OpenMonth: "2026-02"})

	decidertest.Given(t, domain.Account, reviewed()...).
		When(reopen).
		Then(domain.OpeningBalanceReversed{TransactionID: openingTransaction, ReversalID: newReversal, Amount: dec("-100.00")})
}
