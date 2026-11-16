package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	earlierSalary = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000101")
	earlierCoffee = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000102")
)

func described(r domain.StatementRow, description string) domain.StatementRow {
	r.Description = description

	return r
}

func earlier(id sharedkernel.ID, date, amount, bankID, description string) domain.ImportedKey {
	k := key(date, amount, bankID)
	k.ID, k.Description = id, description

	return k
}

// A row that only looks like an earlier one is a Suspected duplicate and
// records the earlier Imported transaction it resembles: the same bank
// transaction ID with another Booking date or amount; or, without a bank
// transaction ID, the same Booking date, amount and normalised
// description. Rows of the file are never compared with each other (spec
// stories 39, 40e).
func TestRowsThatOnlyLookLikeEarlierOnesAreSuspectedDuplicates(t *testing.T) {
	t.Parallel()

	s := domain.Statement{Rows: []domain.StatementRow{
		described(statementRow(2, "2026-01-16", "3000.00", "T-1"), "Salary"),      // same ID, another date
		described(statementRow(3, "2026-01-15", "3000.01", "T-1"), "Salary"),      // same ID, another amount
		described(statementRow(4, "2026-02-02", "-10.5", ""), "  COFFEE   shop "), // no ID, same day, amount, description
		described(statementRow(5, "2026-02-02", "-10.50", ""), "Tea shop"),        // another description
		described(statementRow(6, "2026-02-03", "-10.50", ""), "Coffee shop"),     // another day
		described(statementRow(7, "2026-02-02", "-10.50", "T-7"), "Coffee shop"),  // a new bank ID: not compared by description
		described(statementRow(8, "2026-02-09", "-2.00", ""), "Bus"),              // new
		described(statementRow(9, "2026-02-09", "-2.00", ""), "Bus"),              // the same again within the file
	}, Total: dec("5954.01")}

	got := s.WithSuspectedDuplicates([]domain.ImportedKey{
		earlier(earlierSalary, "2026-01-15", "3000.00", "T-1", "Salary"),
		earlier(earlierCoffee, "2026-02-02", "-10.50", "", "Coffee Shop"),
	})

	want := map[int]sharedkernel.ID{2: earlierSalary, 3: earlierSalary, 4: earlierCoffee}

	for _, r := range got.Rows {
		if r.Resembles != want[r.Line] {
			t.Errorf("line %d resembles %s, want %s", r.Line, r.Resembles, want[r.Line])
		}
	}

	if len(got.Rows) != len(s.Rows) || got.Suspected != 3 || !got.Total.Equal(s.Total) {
		t.Errorf("statement = %d rows, %d suspected, total %s; want every row kept, 3 suspected, the same total",
			len(got.Rows), got.Suspected, got.Total)
	}
}

// waitingImport is laterImport with one Suspected duplicate of -10.50
// among its rows, and an Export balance that its rows only meet if the
// Suspected duplicate is Discarded: 510.50 + 1989.50 - 45.10 = 2454.90.
func waitingImport() domain.RecordStatementImport {
	c := laterImport()
	c.RowsImported, c.SuspectedDuplicates = 2, 1
	c.RowsTotal = dec("-55.60")

	return c
}

func waitingRecorded() domain.StatementImportRecorded {
	return domain.StatementImportRecorded{
		ConnectionID:        theConnection,
		AccountID:           theAccount,
		FileHash:            "def456",
		MappingVersion:      2,
		Currency:            "EUR",
		ExportBalance:       dec("2454.90"),
		RowsImported:        2,
		RowsSkipped:         2,
		SuspectedDuplicates: 1,
		RowsTotal:           dec("-55.60"),
		BalanceCheck:        domain.BalanceCheckWaiting,
	}
}

// A later import that produced Suspected duplicates is recorded with its
// Balance check waiting, even though the check would fail now: the check
// runs once every Suspected duplicate is decided (spec story 43c).
func TestALaterImportWithSuspectedDuplicatesIsRecordedWithItsCheckWaiting(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.StatementImport).When(waitingImport()).Then(waitingRecorded())

	s := decidertest.Given(t, domain.StatementImport, domain.StatementImportEvent(waitingRecorded())).State()
	if s.BalanceCheck != domain.BalanceCheckWaiting || string(s.BalanceCheck) != "waiting" || s.SuspectedDuplicates != 1 {
		t.Errorf("state = %+v, want the check waiting on 1 Suspected duplicate", s)
	}
}

// Once the last Suspected duplicate is decided, the waiting check runs on
// the Account's Opening balance and its non-Discarded Imported
// transactions up to this import, and is recorded as passed or failed.
func TestAWaitingBalanceCheckRunsOnceTheSuspectedDuplicatesAreDecided(t *testing.T) {
	t.Parallel()

	waiting := []domain.StatementImportEvent{waitingRecorded()}

	// The Suspected duplicate was Discarded: 510.50 + 1989.50 - 45.10.
	decidertest.Given(t, domain.StatementImport, waiting...).
		When(domain.RunBalanceCheck{OpeningBalance: dec("510.50"), Total: dec("1944.40")}).
		Then(domain.WaitingCheckPassed{})

	// The Suspected duplicate was kept: 10.50 short of the Export balance.
	decidertest.Given(t, domain.StatementImport, waiting...).
		When(domain.RunBalanceCheck{OpeningBalance: dec("510.50"), Total: dec("1933.90")}).
		Then(domain.WaitingCheckFailed{Expected: dec("2444.40"), Difference: dec("10.50")})

	passed := decidertest.Given(t, domain.StatementImport, append(waiting, domain.WaitingCheckPassed{})...).State()
	if passed.BalanceCheck != domain.BalanceCheckPassed {
		t.Errorf("check = %q, want passed", passed.BalanceCheck)
	}

	failed := decidertest.Given(t, domain.StatementImport,
		append(waiting, domain.WaitingCheckFailed{Expected: dec("2444.40"), Difference: dec("10.50")})...).State()
	if failed.BalanceCheck != domain.BalanceCheckFailed || string(failed.BalanceCheck) != "failed" {
		t.Errorf("check = %q, want failed", failed.BalanceCheck)
	}
}

// Only a waiting check runs.
func TestRunningABalanceCheckThatIsNotWaitingIsRefused(t *testing.T) {
	t.Parallel()

	run := domain.RunBalanceCheck{OpeningBalance: dec("510.50"), Total: dec("1944.40")}

	decidertest.Given(t, domain.StatementImport).When(run).ThenError(domain.ErrBalanceCheckNotWaiting)

	passed, err := domain.StatementImport.Decide(domain.StatementImport.Initial, laterImport())
	if err != nil {
		t.Fatal(err)
	}

	decidertest.Given(t, domain.StatementImport, passed...).When(run).ThenError(domain.ErrBalanceCheckNotWaiting)
	decidertest.Given(t, domain.StatementImport, []domain.StatementImportEvent{waitingRecorded(), domain.WaitingCheckPassed{}}...).
		When(run).ThenError(domain.ErrBalanceCheckNotWaiting)

	// A voided import's waiting check never runs.
	decidertest.Given(t, domain.StatementImport, []domain.StatementImportEvent{waitingRecorded(), domain.StatementImportVoided{}}...).
		When(run).ThenError(domain.ErrBalanceCheckNotWaiting)
}

// Voiding an import Discards its Suspected duplicates and the ones kept
// (spec story 43d).
func TestVoidingDiscardsSuspectedDuplicatesAndKeptRows(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.ImportedTransaction, suspectedRow(t)...).
		When(domain.DiscardForVoid{}).
		Then(domain.ImportedTransactionDiscarded{})

	decidertest.Given(t, domain.ImportedTransaction, append(suspectedRow(t), domain.ImportedTransactionKept{})...).
		When(domain.DiscardForVoid{}).
		Then(domain.ImportedTransactionDiscarded{})
}

func suspectedRow(t *testing.T) []domain.ImportedTransactionEvent {
	t.Helper()

	row := importRow()
	row.Resembles = earlierCoffee

	events, err := domain.ImportedTransaction.Decide(domain.ImportedTransaction.Initial, row)
	if err != nil {
		t.Fatal(err)
	}

	return events
}

// A row that resembles an earlier Imported transaction is imported as a
// Suspected duplicate, not pending, and records the one it resembles (spec
// stories 39, 40b).
func TestImportingARowThatResemblesAnEarlierOneMakesASuspectedDuplicate(t *testing.T) {
	t.Parallel()

	row := importRow()
	row.Resembles = earlierCoffee

	decidertest.Given(t, domain.ImportedTransaction).
		When(row).
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
			Resembles:         earlierCoffee,
		})

	s := decidertest.Given(t, domain.ImportedTransaction, suspectedRow(t)...).State()
	if s.State != domain.ImportedSuspectedDuplicate || string(s.State) != "suspected_duplicate" || s.Resembles != earlierCoffee {
		t.Errorf("state = %q resembling %s, want suspected_duplicate resembling %s", s.State, s.Resembles, earlierCoffee)
	}
}

// Keeping a Suspected duplicate makes it pending, to be Accepted like any
// other row; it still records what it resembled (spec story 40c).
func TestKeepingASuspectedDuplicateMakesItPending(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.ImportedTransaction, suspectedRow(t)...).
		When(domain.KeepImportedTransaction{}).
		Then(domain.ImportedTransactionKept{})

	kept := append(suspectedRow(t), domain.ImportedTransactionKept{})
	s := decidertest.Given(t, domain.ImportedTransaction, kept...).State()

	if s.State != domain.ImportedPending || s.Resembles != earlierCoffee {
		t.Errorf("state = %q resembling %s, want pending, still resembling %s", s.State, s.Resembles, earlierCoffee)
	}

	decidertest.Given(t, domain.ImportedTransaction, kept...).
		When(domain.AcceptImportedTransaction{TransactionID: theTransaction}).
		Then(domain.ImportedTransactionAccepted{TransactionID: theTransaction})
}

// Discarding a Suspected duplicate confirms it as a repeat: it is final
// (spec story 40d).
func TestDiscardingASuspectedDuplicate(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.ImportedTransaction, suspectedRow(t)...).
		When(domain.DiscardImportedTransaction{}).
		Then(domain.ImportedTransactionDiscarded{})
}

// Only a Suspected duplicate is kept, and one is decided before it can be
// Accepted.
func TestSuspectedDuplicateRefusals(t *testing.T) {
	t.Parallel()

	keep := domain.KeepImportedTransaction{}

	decidertest.Given(t, domain.ImportedTransaction).When(keep).ThenError(domain.ErrImportedTransactionNotImported)
	decidertest.Given(t, domain.ImportedTransaction, imported(t)...).When(keep).
		ThenError(domain.ErrImportedTransactionNotSuspectedDuplicate)
	decidertest.Given(t, domain.ImportedTransaction, acceptedRow(t)...).When(keep).
		ThenError(domain.ErrImportedTransactionNotSuspectedDuplicate)
	decidertest.Given(t, domain.ImportedTransaction, append(suspectedRow(t), domain.ImportedTransactionKept{})...).
		When(keep).ThenError(domain.ErrImportedTransactionNotSuspectedDuplicate)
	decidertest.Given(t, domain.ImportedTransaction, append(suspectedRow(t), domain.ImportedTransactionDiscarded{})...).
		When(keep).ThenError(domain.ErrImportedTransactionNotSuspectedDuplicate)

	decidertest.Given(t, domain.ImportedTransaction, suspectedRow(t)...).
		When(domain.AcceptImportedTransaction{TransactionID: theTransaction}).
		ThenError(domain.ErrImportedTransactionSuspectedDuplicate)
}
