package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var openingReversal = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000ab")

// Voiding the first Statement import reverses the Opening balance
// Transaction, the one Transaction a User can't reverse (ADR-0018).
func TestVoidingReversesTheOpeningBalanceTransaction(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction, openingRecorded(t)...).
		When(domain.ReverseOpeningBalance{ReversalID: openingReversal}).
		Then(domain.TransactionReversed{ReversalID: openingReversal})

	reversed := append(openingRecorded(t), domain.TransactionReversed{ReversalID: openingReversal})
	s := decidertest.Given(t, domain.Transaction, reversed...).State()

	record := s.Reversal(sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000aa"))
	if record.OpeningBalance || record.Postings[0].Amount.String() != "-100" {
		t.Errorf("reversal = %+v, want the opposite Postings, not itself an Opening balance", record)
	}

	decidertest.Given(t, domain.Transaction, reversed...).
		When(domain.ReverseOpeningBalance{ReversalID: anotherReversal}).
		ThenError(domain.ErrTransactionAlreadyReversed)
}

func TestReversingTheOpeningBalanceRefusals(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction).
		When(domain.ReverseOpeningBalance{ReversalID: openingReversal}).
		ThenError(domain.ErrTransactionNotRecorded)

	decidertest.Given(t, domain.Transaction, coffeeRecorded()).
		When(domain.ReverseOpeningBalance{ReversalID: openingReversal}).
		ThenError(domain.ErrNotTheOpeningBalanceTransaction)

	decidertest.Given(t, domain.Transaction, openingRecorded(t)...).
		When(domain.ReverseOpeningBalance{}).
		ThenError(domain.ErrReversalIDRequired)
}

func withOpeningBalance() []domain.AccountEvent {
	return []domain.AccountEvent{
		mainAdded,
		domain.OpeningBalanceRecorded{TransactionID: openingTransaction, Amount: dec("1000.50")},
	}
}

// Reopening the Opening balance moves the balance back and sets the Account
// awaiting its first Statement import again, so a corrected first file
// works it out again (spec story 43e).
func TestReopeningTheOpeningBalance(t *testing.T) {
	t.Parallel()

	reopen := domain.ReopenOpeningBalance{
		TransactionID: openingTransaction, ReversalID: openingReversal, Amount: dec("-1000.50"),
	}

	decidertest.Given(t, domain.Account, withOpeningBalance()...).
		When(reopen).
		Then(domain.OpeningBalanceReversed{
			TransactionID: openingTransaction, ReversalID: openingReversal, Amount: dec("-1000.50"),
		})

	reopened := append(withOpeningBalance(), domain.OpeningBalanceReversed{
		TransactionID: openingTransaction, ReversalID: openingReversal, Amount: dec("-1000.50"),
	})
	s := decidertest.Given(t, domain.Account, reopened...).State()

	if !s.AwaitingFirstStatementImport || s.HasBalance() || s.OpeningBalanceTransactionID != sharedkernel.NilID {
		t.Errorf("account = %+v, want it awaiting its first Statement import again", s)
	}

	if !s.Balance().IsZero() {
		t.Errorf("balance = %s, want the Opening balance moved back", s.Balance())
	}

	// A corrected first import records a new Opening balance.
	decidertest.Given(t, domain.Account, reopened...).
		When(domain.RecordOpeningBalance{TransactionID: openingReversal, Amount: dec("7")}).
		Then(domain.OpeningBalanceRecorded{TransactionID: openingReversal, Amount: dec("7")})
}

func TestReopeningTheOpeningBalanceRefusals(t *testing.T) {
	t.Parallel()

	reopen := domain.ReopenOpeningBalance{
		TransactionID: openingTransaction, ReversalID: openingReversal, Amount: dec("-1000.50"),
	}

	decidertest.Given(t, domain.Account).When(reopen).ThenError(domain.ErrAccountNotAdded)
	decidertest.Given(t, domain.Account, mainAdded).When(reopen).
		ThenError(domain.ErrAwaitingFirstStatementImport)

	other := reopen
	other.TransactionID = coffeeReversal
	decidertest.Given(t, domain.Account, withOpeningBalance()...).When(other).
		ThenError(domain.ErrNotTheOpeningBalanceTransaction)
}
