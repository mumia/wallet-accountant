package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var mainAdded = domain.AccountEvent(domain.AccountAdded{
	Name: "Main", Kind: domain.AccountKindBankAccount, Currency: "EUR", StartDate: "2026-01-15",
})

var openingTransaction = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000aa")

func TestTheFirstStatementImportRecordsTheOpeningBalance(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Account, mainAdded).
		When(domain.RecordOpeningBalance{TransactionID: openingTransaction, Amount: dec("-250.10")}).
		Then(domain.OpeningBalanceRecorded{TransactionID: openingTransaction, Amount: dec("-250.10")})
}

// The Opening balance sets the balance on the start date: the start date's
// Booking month moves by it, and the Account stops awaiting its first
// Statement import.
func TestTheOpeningBalanceIsTheAccountsFirstBalance(t *testing.T) {
	t.Parallel()

	s := decidertest.Given(t, domain.Account, mainAdded,
		domain.AccountEvent(domain.OpeningBalanceRecorded{TransactionID: openingTransaction, Amount: dec("1000.5")})).State()

	if s.AwaitingFirstStatementImport {
		t.Error("still awaiting its first Statement import")
	}

	if !s.HasBalance() || !s.Balance().Equal(dec("1000.50")) {
		t.Errorf("balance = %s (has one: %v), want 1000.50", s.Balance(), s.HasBalance())
	}

	if got := s.Months["2026-01"]; !got.Equal(dec("1000.5")) || len(s.Months) != 1 {
		t.Errorf("months = %v, want 2026-01 moved by 1000.5", s.Months)
	}

	if s.OpeningBalanceTransactionID != openingTransaction {
		t.Errorf("opening balance transaction = %s", s.OpeningBalanceTransactionID)
	}
}

func TestRecordingTheOpeningBalanceRefusals(t *testing.T) {
	t.Parallel()

	record := domain.RecordOpeningBalance{TransactionID: openingTransaction, Amount: dec("10")}

	decidertest.Given(t, domain.Account).When(record).ThenError(domain.ErrAccountNotAdded)

	decidertest.Given(t, domain.Account, mainAdded,
		domain.AccountEvent(domain.OpeningBalanceRecorded{TransactionID: openingTransaction, Amount: dec("5")})).
		When(record).
		ThenError(domain.ErrOpeningBalanceAlreadyRecorded)

	decidertest.Given(t, domain.Account, mainAdded).
		When(domain.RecordOpeningBalance{TransactionID: openingTransaction, Amount: dec("0.001")}).
		ThenError(domain.ErrTooManyDecimalPlaces)
}
