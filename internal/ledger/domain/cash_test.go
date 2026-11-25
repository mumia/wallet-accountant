package domain_test

import (
	"slices"
	"testing"

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
