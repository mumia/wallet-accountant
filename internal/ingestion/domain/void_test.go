package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

func recordedImport(t *testing.T) []domain.StatementImportEvent {
	t.Helper()

	events, err := domain.StatementImport.Decide(domain.StatementImport.Initial, firstImport())
	if err != nil {
		t.Fatal(err)
	}

	return events
}

// The Account's latest import with no Accepted rows is voided, and its fold
// says so (spec story 43d, ADR-0018).
func TestVoidingTheLatestStatementImport(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.StatementImport, recordedImport(t)...).
		When(domain.VoidStatementImport{Latest: true}).
		Then(domain.StatementImportVoided{})

	s := decidertest.Given(t, domain.StatementImport,
		append(recordedImport(t), domain.StatementImportVoided{})...).State()

	if !s.Voided() || s.VoidedAt.IsZero() {
		t.Errorf("voided = %v at %v, want voided with its time", s.Voided(), s.VoidedAt)
	}

	if s.OpeningBalance == nil {
		t.Error("a voided first import still tells it had an Opening balance, so the void can reverse it")
	}
}

// Only the latest import, with no Accepted rows, can be voided, and only
// once (spec stories 43d, 43f).
func TestVoidingAStatementImportRefusals(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.StatementImport).
		When(domain.VoidStatementImport{Latest: true}).
		ThenError(domain.ErrStatementImportNotRecorded)

	decidertest.Given(t, domain.StatementImport, recordedImport(t)...).
		When(domain.VoidStatementImport{Latest: false}).
		ThenError(domain.ErrNotTheLatestStatementImport)

	decidertest.Given(t, domain.StatementImport, recordedImport(t)...).
		When(domain.VoidStatementImport{Latest: true, AcceptedRows: 1}).
		ThenError(domain.ErrStatementImportHasAcceptedRows)

	decidertest.Given(t, domain.StatementImport, append(recordedImport(t), domain.StatementImportVoided{})...).
		When(domain.VoidStatementImport{Latest: true}).
		ThenError(domain.ErrStatementImportAlreadyVoided)
}

// Voiding Discards every row of the import that isn't Accepted; a row
// already Discarded stays as it is (spec "Imported transaction" states).
func TestVoidingDiscardsAnImportsRows(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.ImportedTransaction, imported(t)...).
		When(domain.DiscardForVoid{}).
		Then(domain.ImportedTransactionDiscarded{})

	returned := append(acceptedRow(t), domain.ImportedTransactionReturnedToPending{TransactionID: theTransaction})
	decidertest.Given(t, domain.ImportedTransaction, returned...).
		When(domain.DiscardForVoid{}).
		Then(domain.ImportedTransactionDiscarded{})

	discarded := append(imported(t), domain.ImportedTransactionDiscarded{})
	decidertest.Given(t, domain.ImportedTransaction, discarded...).
		When(domain.DiscardForVoid{}).
		Then()

	decidertest.Given(t, domain.ImportedTransaction, acceptedRow(t)...).
		When(domain.DiscardForVoid{}).
		ThenError(domain.ErrImportedTransactionAlreadyAccepted)

	decidertest.Given(t, domain.ImportedTransaction).
		When(domain.DiscardForVoid{}).
		ThenError(domain.ErrImportedTransactionNotImported)
}
