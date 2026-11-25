package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	walletAdded = domain.AccountEvent(domain.AccountAdded{
		Name: "Wallet", Kind: domain.AccountKindCash, Currency: "EUR", StartDate: "2026-01-01",
	})
	walletOpening   = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000d1")
	walletRecount   = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000d2")
	walletReversal  = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000d3")
	walletWithCount = []domain.AccountEvent{
		walletAdded,
		domain.OpeningBalanceRecorded{TransactionID: walletOpening, Amount: dec("40.00")},
	}
)

func TestAddingACashAccountWithItsOpeningCount(t *testing.T) {
	t.Parallel()

	for name, count := range map[string]string{"some cash": "40.00", "an empty wallet": "0"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Account).
				When(domain.AddAccount{
					Name: "Wallet", Kind: domain.AccountKindCash, Currency: "EUR", StartDate: startDate,
					OpeningCount: new(dec(count)),
				}).
				Then(walletAdded)
		})
	}
}

// The Opening count records the Opening balance at once, so a cash Account
// never awaits a Statement import (ADR-0018).
func TestACashAccountHasABalanceFromItsOpeningCount(t *testing.T) {
	t.Parallel()

	s := decidertest.Given(t, domain.Account, walletWithCount...).State()

	if !s.IsCash() || s.AwaitingFirstStatementImport || !s.HasBalance() || !s.Balance().Equal(dec("40")) {
		t.Errorf("state = %+v, want a cash Account with a balance of 40", s)
	}
}

func TestAddingACashAccountRefusals(t *testing.T) {
	t.Parallel()

	cash := domain.AddAccount{Name: "Wallet", Kind: domain.AccountKindCash, Currency: "EUR", StartDate: startDate}

	decidertest.Given(t, domain.Account).When(cash).ThenError(domain.ErrOpeningCountRequired)

	tooPrecise := cash
	tooPrecise.OpeningCount = new(dec("40.001"))
	decidertest.Given(t, domain.Account).When(tooPrecise).ThenError(domain.ErrTooManyDecimalPlaces)

	statement := domain.AddAccount{
		Name: "Main", Kind: domain.AccountKindBankAccount, Currency: "EUR", StartDate: startDate,
		OpeningCount: new(dec("40.00")),
	}
	decidertest.Given(t, domain.Account).When(statement).ThenError(domain.ErrOpeningCountNotForStatementAccount)
}

func TestRecountingTheOpeningCount(t *testing.T) {
	t.Parallel()

	recount := domain.RecountOpening{
		TransactionID: walletRecount, ReversalID: walletReversal, PreviousAmount: dec("40.00"), Amount: dec("35.50"),
	}

	decidertest.Given(t, domain.Account, walletWithCount...).
		When(recount).
		Then(
			domain.OpeningBalanceReversed{
				TransactionID: walletOpening, ReversalID: walletReversal, Amount: dec("-40.00"),
			},
			domain.OpeningBalanceRecorded{TransactionID: walletRecount, Amount: dec("35.50")},
		)

	s := decidertest.Given(t, domain.Account, slices.Concat(walletWithCount, []domain.AccountEvent{
		domain.OpeningBalanceReversed{TransactionID: walletOpening, ReversalID: walletReversal, Amount: dec("-40.00")},
		domain.OpeningBalanceRecorded{TransactionID: walletRecount, Amount: dec("35.50")},
	})...).State()

	if s.AwaitingFirstStatementImport || s.OpeningBalanceTransactionID != walletRecount || !s.Balance().Equal(dec("35.5")) {
		t.Errorf("state = %+v, want the new Opening balance of 35.50", s)
	}
}

func TestRecountingTheSameOpeningCountRecordsNothing(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Account, walletWithCount...).
		When(domain.RecountOpening{
			TransactionID: walletRecount, ReversalID: walletReversal, PreviousAmount: dec("40.00"), Amount: dec("40"),
		}).
		Then()
}

func TestRecountingTheOpeningCountRefusals(t *testing.T) {
	t.Parallel()

	recount := domain.RecountOpening{
		TransactionID: walletRecount, ReversalID: walletReversal, PreviousAmount: dec("40.00"), Amount: dec("35.50"),
	}

	decidertest.Given(t, domain.Account).When(recount).ThenError(domain.ErrAccountNotAdded)

	decidertest.Given(t, domain.Account, withOpeningBalance()...).
		When(recount).
		ThenError(domain.ErrNotACashAccount)

	startMonthClosed := slices.Concat(walletWithCount,
		[]domain.AccountEvent{domain.MonthClosed{Month: "2026-01", StatementBalance: dec("40.00")}})
	decidertest.Given(t, domain.Account, startMonthClosed...).
		When(recount).
		ThenError(domain.ErrOpeningCountLocked)

	tooPrecise := recount
	tooPrecise.Amount = dec("35.555")
	decidertest.Given(t, domain.Account, walletWithCount...).
		When(tooPrecise).
		ThenError(domain.ErrTooManyDecimalPlaces)
}

// A cash Account is kept by hand, so it books in its Open month and any
// later one, but never in a closed month or before its start date
// (ADR-0024; spec stories 7 and 8).
func TestBookingOnACashAccount(t *testing.T) {
	t.Parallel()

	januaryClosed := slices.Concat(walletWithCount,
		[]domain.AccountEvent{domain.MonthClosed{Month: "2026-01", StatementBalance: dec("40.00")}})

	for name, tc := range map[string]struct {
		past []domain.AccountEvent
		date time.Time
	}{
		"the Open month": {walletWithCount, jan20},
		"a later month":  {walletWithCount, time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)},
		"after a close":  {januaryClosed, time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Account, tc.past...).
				When(domain.BookTransaction{
					TransactionID: coffeeBooked, BookingDate: tc.date, Amount: dec("-2.50"), Uncategorised: true,
				}).
				Then(domain.TransactionBooked{
					TransactionID: coffeeBooked, BookingDate: tc.date.Format(domain.DateLayout), Amount: dec("-2.50"),
					Uncategorised: true,
				})
		})
	}

	book := domain.BookTransaction{TransactionID: coffeeBooked, BookingDate: jan20, Amount: dec("-2.50")}
	decidertest.Given(t, domain.Account, januaryClosed...).
		When(book).
		ThenError(domain.ErrMonthClosed)

	early := book
	early.BookingDate = time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)
	decidertest.Given(t, domain.Account, walletWithCount...).When(early).ThenError(domain.ErrBookedBeforeStartDate)
}

// A cash Transaction booked after the Open month is reversed in its own
// month, which is still open; a closed month refuses its reversal.
func TestReversingOnACashAccount(t *testing.T) {
	t.Parallel()

	march3 := time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)
	booked := slices.Concat(walletWithCount, []domain.AccountEvent{
		domain.TransactionBooked{TransactionID: coffeeBooked, BookingDate: "2026-03-03", Amount: dec("-2.50")},
	})
	reverse := domain.BookReversal{
		TransactionID: coffeeBooked, ReversalID: salaryBooked, BookingDate: march3, Amount: dec("2.50"),
	}

	decidertest.Given(t, domain.Account, booked...).
		When(reverse).
		Then(domain.ReversalBooked{
			TransactionID: coffeeBooked, ReversalID: salaryBooked, BookingDate: "2026-03-03", Amount: dec("2.50"),
		})

	inJanuary := reverse
	inJanuary.BookingDate = jan20
	decidertest.Given(t, domain.Account, slices.Concat(walletWithCount, []domain.AccountEvent{
		domain.TransactionBooked{TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-2.50")},
		domain.MonthClosed{Month: "2026-01", StatementBalance: dec("37.50")},
	})...).
		When(inJanuary).
		ThenError(domain.ErrMonthClosed)
}
