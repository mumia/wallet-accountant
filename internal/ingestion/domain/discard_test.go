package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

func acceptedRow(t *testing.T) []domain.ImportedTransactionEvent {
	t.Helper()

	return append(imported(t), domain.ImportedTransactionAccepted{TransactionID: theTransaction})
}

// Reversing its Transaction returns an Accepted Imported transaction to
// pending, unlinked, so it can be Accepted again (spec story 51d).
func TestReversingReturnsAnAcceptedImportedTransactionToPending(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.ImportedTransaction, acceptedRow(t)...).
		When(domain.ReturnToPending{TransactionID: theTransaction}).
		Then(domain.ImportedTransactionReturnedToPending{TransactionID: theTransaction})

	returned := append(acceptedRow(t), domain.ImportedTransactionReturnedToPending{TransactionID: theTransaction})
	s := decidertest.Given(t, domain.ImportedTransaction, returned...).State()

	if s.State != domain.ImportedPending || s.TransactionID != sharedkernel.NilID {
		t.Errorf("state = %s booked by %s, want pending and unbooked", s.State, s.TransactionID)
	}

	decidertest.Given(t, domain.ImportedTransaction, returned...).
		When(domain.AcceptImportedTransaction{TransactionID: otherTransaction}).
		Then(domain.ImportedTransactionAccepted{TransactionID: otherTransaction})
}

// Only the Transaction that booked it returns it to pending.
func TestReturningToPendingRefusals(t *testing.T) {
	t.Parallel()

	back := domain.ReturnToPending{TransactionID: theTransaction}

	decidertest.Given(t, domain.ImportedTransaction).When(back).ThenError(domain.ErrImportedTransactionNotImported)
	decidertest.Given(t, domain.ImportedTransaction, imported(t)...).When(back).
		ThenError(domain.ErrImportedTransactionNotAccepted)
	decidertest.Given(t, domain.ImportedTransaction, acceptedRow(t)...).
		When(domain.ReturnToPending{TransactionID: otherTransaction}).
		ThenError(domain.ErrNotBookedByTransaction)
}

// A pending Imported transaction can be Discarded, and Discarded is final
// (spec story 51e).
func TestDiscardingAPendingImportedTransaction(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.ImportedTransaction, imported(t)...).
		When(domain.DiscardImportedTransaction{}).
		Then(domain.ImportedTransactionDiscarded{})

	discarded := append(imported(t), domain.ImportedTransactionDiscarded{})
	s := decidertest.Given(t, domain.ImportedTransaction, discarded...).State()

	if s.State != domain.ImportedDiscarded || string(s.State) != "discarded" {
		t.Errorf("state = %q, want discarded", s.State)
	}

	decidertest.Given(t, domain.ImportedTransaction, discarded...).
		When(domain.DiscardImportedTransaction{}).ThenError(domain.ErrImportedTransactionAlreadyDiscarded)
	decidertest.Given(t, domain.ImportedTransaction, discarded...).
		When(domain.AcceptImportedTransaction{TransactionID: theTransaction}).
		ThenError(domain.ErrImportedTransactionDiscarded)

	// Discard after reversal (spec story 51d).
	returned := append(acceptedRow(t), domain.ImportedTransactionReturnedToPending{TransactionID: theTransaction})
	decidertest.Given(t, domain.ImportedTransaction, returned...).
		When(domain.DiscardImportedTransaction{}).
		Then(domain.ImportedTransactionDiscarded{})
}

// An Accepted Imported transaction is in the Ledger: it is reversed, not
// Discarded.
func TestDiscardingRefusals(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.ImportedTransaction).
		When(domain.DiscardImportedTransaction{}).ThenError(domain.ErrImportedTransactionNotImported)
	decidertest.Given(t, domain.ImportedTransaction, acceptedRow(t)...).
		When(domain.DiscardImportedTransaction{}).ThenError(domain.ErrImportedTransactionAlreadyAccepted)
}
