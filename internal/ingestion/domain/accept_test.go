package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	theTransaction   = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000010")
	otherTransaction = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000011")
)

func imported(t *testing.T) []domain.ImportedTransactionEvent {
	t.Helper()

	events, err := domain.ImportedTransaction.Decide(domain.ImportedTransaction.Initial, importRow())
	if err != nil {
		t.Fatal(err)
	}

	return events
}

// Accepting a pending Imported transaction links it to the Ledger
// Transaction that books it (spec stories 45, 46).
func TestAcceptingAPendingImportedTransaction(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.ImportedTransaction, imported(t)...).
		When(domain.AcceptImportedTransaction{TransactionID: theTransaction}).
		Then(domain.ImportedTransactionAccepted{TransactionID: theTransaction})

	accepted := append(imported(t), domain.ImportedTransactionAccepted{TransactionID: theTransaction})
	s := decidertest.Given(t, domain.ImportedTransaction, accepted...).State()

	if s.State != domain.ImportedAccepted || s.TransactionID != theTransaction {
		t.Errorf("state = %s booked by %s, want accepted and booked by %s", s.State, s.TransactionID, theTransaction)
	}

	if !s.Amount.Equal(dec("-12.30")) || s.Description != "Coffee" || !s.BookingDate.Equal(bookedOn) {
		t.Errorf("accepting changed the row: %+v", s)
	}
}

// An Imported transaction is booked once: accepting it again is refused
// (spec story 48), and so is accepting one that was never imported.
func TestAcceptingRefusals(t *testing.T) {
	t.Parallel()

	accepted := append(imported(t), domain.ImportedTransactionAccepted{TransactionID: theTransaction})

	decidertest.Given(t, domain.ImportedTransaction, accepted...).
		When(domain.AcceptImportedTransaction{TransactionID: otherTransaction}).
		ThenError(domain.ErrImportedTransactionAlreadyAccepted)

	decidertest.Given(t, domain.ImportedTransaction).
		When(domain.AcceptImportedTransaction{TransactionID: theTransaction}).
		ThenError(domain.ErrImportedTransactionNotImported)

	decidertest.Given(t, domain.ImportedTransaction, imported(t)...).
		When(domain.AcceptImportedTransaction{}).
		ThenError(domain.ErrTransactionIDRequired)
}
