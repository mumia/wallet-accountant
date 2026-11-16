package domain_test

import (
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	theAccount    = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000001")
	theConnection = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000002")
	theImport     = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000003")
)

func firstImport() domain.RecordStatementImport {
	return domain.RecordStatementImport{
		ConnectionID:   theConnection,
		AccountID:      theAccount,
		FileHash:       "abc123",
		MappingVersion: 2,
		Currency:       "EUR",
		ExportBalance:  dec("2500.00"),
		RowsImported:   2,
		RowsIgnored:    1,
		RowsTotal:      dec("1989.50"),
		First:          true,
	}
}

// The first import works out the Opening balance from its Export balance,
// so its Balance check passes by construction (ADR-0018).
func TestRecordingAnAccountsFirstStatementImport(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.StatementImport).
		When(firstImport()).
		Then(domain.StatementImportRecorded{
			ConnectionID:   theConnection,
			AccountID:      theAccount,
			FileHash:       "abc123",
			MappingVersion: 2,
			Currency:       "EUR",
			ExportBalance:  dec("2500.00"),
			RowsImported:   2,
			RowsIgnored:    1,
			RowsTotal:      dec("1989.50"),
			OpeningBalance: new(dec("2500.00").Sub(dec("1989.50"))),
			BalanceCheck:   domain.BalanceCheckPassed,
		})
}

func TestRecordingAStatementImportRefusals(t *testing.T) {
	t.Parallel()

	recorded, err := domain.StatementImport.Decide(domain.StatementImport.Initial, firstImport())
	if err != nil {
		t.Fatal(err)
	}

	decidertest.Given(t, domain.StatementImport, recorded...).
		When(firstImport()).
		ThenError(domain.ErrStatementImportAlreadyRecorded)

	noHash := firstImport()
	noHash.FileHash = ""
	decidertest.Given(t, domain.StatementImport).When(noHash).ThenError(domain.ErrFileHashRequired)

	tooPrecise := firstImport()
	tooPrecise.ExportBalance = dec("1.001")
	decidertest.Given(t, domain.StatementImport).When(tooPrecise).ThenError(domain.ErrInvalidExportBalance)

	later := firstImport()
	later.First = false
	decidertest.Given(t, domain.StatementImport).When(later).ThenError(domain.ErrLaterImportsNotSupported)
}

func TestTheFoldKeepsWhatTheImportRecorded(t *testing.T) {
	t.Parallel()

	recorded, err := domain.StatementImport.Decide(domain.StatementImport.Initial, firstImport())
	if err != nil {
		t.Fatal(err)
	}

	s := decidertest.Given(t, domain.StatementImport, recorded...).State()

	if !s.Recorded || s.AccountID != theAccount || s.ConnectionID != theConnection || s.FileHash != "abc123" ||
		s.RowsImported != 2 || s.RowsIgnored != 1 || s.OpeningBalance == nil || !s.OpeningBalance.Equal(dec("510.50")) ||
		s.BalanceCheck != domain.BalanceCheckPassed || s.RecordedAt.IsZero() {
		t.Errorf("state = %+v", s)
	}
}

var bookedOn = time.Date(2026, time.January, 20, 0, 0, 0, 0, time.UTC)

func importRow() domain.ImportRow {
	return domain.ImportRow{
		StatementImportID: theImport,
		AccountID:         theAccount,
		Line:              3,
		BookingDate:       bookedOn,
		TransactionDate:   bookedOn.AddDate(0, 0, -1),
		Amount:            dec("-12.30"),
		Currency:          "EUR",
		Description:       "Coffee",
		Payee:             "Café",
		BankTransactionID: "T-1",
	}
}

// Every row of an import becomes a pending Imported transaction in the
// Account currency (spec stories 34, 35).
func TestImportingARowMakesAPendingImportedTransaction(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.ImportedTransaction).
		When(importRow()).
		Then(domain.TransactionImported{
			StatementImportID: theImport,
			AccountID:         theAccount,
			Line:              3,
			BookingDate:       "2026-01-20",
			TransactionDate:   "2026-01-19",
			Amount:            dec("-12.30"),
			Currency:          "EUR",
			Description:       "Coffee",
			Payee:             "Café",
			BankTransactionID: "T-1",
		})

	s := decidertest.Given(t, domain.ImportedTransaction, domain.ImportedTransactionEvent(domain.TransactionImported{
		StatementImportID: theImport, AccountID: theAccount, Line: 3, BookingDate: "2026-01-20",
		TransactionDate: "2026-01-19", ValueDate: "2026-01-21", Amount: dec("-12.30"), Currency: "EUR",
	})).State()

	if s.State != domain.ImportedPending || !s.BookingDate.Equal(bookedOn) || !s.ValueDate.Equal(bookedOn.AddDate(0, 0, 1)) {
		t.Errorf("state = %+v, want pending, booked on 2026-01-20 with its Value date", s)
	}
}

func TestImportingARowRefusals(t *testing.T) {
	t.Parallel()

	recorded, err := domain.ImportedTransaction.Decide(domain.ImportedTransaction.Initial, importRow())
	if err != nil {
		t.Fatal(err)
	}

	decidertest.Given(t, domain.ImportedTransaction, recorded...).
		When(importRow()).
		ThenError(domain.ErrRowAlreadyImported)

	tooPrecise := importRow()
	tooPrecise.Amount = dec("1.234")
	decidertest.Given(t, domain.ImportedTransaction).When(tooPrecise).ThenError(domain.ErrInvalidRowAmount)
}
