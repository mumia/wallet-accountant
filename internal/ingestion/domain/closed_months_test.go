package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

// closedJanuaryFile is a later file read while February is the Open month:
// January rows that repeat, resemble or are new to earlier Imported
// transactions, and February and March rows.
func closedJanuaryFile() domain.Statement {
	return domain.Statement{Rows: []domain.StatementRow{
		statementRow(1, "2026-01-15", "3000.00", "T-1"),                       // a certain repeat of the salary
		statementRow(2, "2026-01-16", "3000.00", "T-1"),                       // a lookalike: same ID, another date
		described(statementRow(3, "2026-01-20", "-10.50", ""), "COFFEE shop"), // a lookalike without an ID
		statementRow(4, "2026-01-25", "-5.00", "T-4"),                         // new, in closed January
		described(statementRow(5, "2026-01-31", "-1.00", ""), "Fee"),          // new, in closed January
		statementRow(6, "2026-02-01", "3000.00", "T-1"),                       // February: a lookalike
		statementRow(7, "2026-02-10", "-20.00", "T-7"),                        // February: new
		statementRow(8, "2026-03-05", "-2.00", "T-8"),                         // March, not open yet
	}, Ignored: 1, Total: dec("8961.50")}
}

func closedJanuaryEarlier() []domain.ImportedKey {
	return []domain.ImportedKey{
		earlier(earlierSalary, "2026-01-15", "3000.00", "T-1", "Salary"),
		earlier(earlierCoffee, "2026-01-20", "-10.50", "", "Coffee Shop"),
	}
}

func lines(rows []domain.StatementRow) []int {
	out := []int{}
	for _, r := range rows {
		out = append(out, r.Line)
	}

	return out
}

// Rows booked before the Open month are split off first and ignored, so
// they are never certain repeats or Suspected duplicates; the rest, in the
// Open month or a month not open yet, are told as before and make the
// total (spec stories 10, 12).
func TestRowsInClosedMonthsAreSplitOffBeforeRepeats(t *testing.T) {
	t.Parallel()

	got := closedJanuaryFile().WithoutClosedMonths("2026-02").
		WithoutCertainRepeats(closedJanuaryEarlier()).WithSuspectedDuplicates(closedJanuaryEarlier())

	if l := lines(got.InClosedMonths); !reflect.DeepEqual(l, []int{1, 2, 3, 4, 5}) {
		t.Errorf("in closed months = lines %v, want every January row", l)
	}

	if l := lines(got.Rows); !reflect.DeepEqual(l, []int{6, 7, 8}) {
		t.Errorf("rows = lines %v, want February's and March's", l)
	}

	if got.Skipped != 0 || got.Suspected != 1 || got.Ignored != 1 || !got.Total.Equal(dec("2978.00")) {
		t.Errorf("statement = %d skipped, %d suspected, %d ignored, total %s; "+
			"want no January repeat skipped, February's lookalike suspected, the one before the start date, 2978.00",
			got.Skipped, got.Suspected, got.Ignored, got.Total)
	}

	if m := got.EarliestBookingMonth(); m != "2026-02" {
		t.Errorf("earliest booking month = %q, want 2026-02", m)
	}
}

// With the start date's month open, nothing is split off.
func TestNothingIsSplitOffWhileNoMonthIsClosed(t *testing.T) {
	t.Parallel()

	got := closedJanuaryFile().WithoutClosedMonths("2026-01")

	if len(got.InClosedMonths) != 0 || len(got.Rows) != 8 || !got.Total.Equal(dec("8961.50")) {
		t.Errorf("statement = %+v, want every row kept", got)
	}

	if m := got.EarliestBookingMonth(); m != "2026-01" {
		t.Errorf("earliest booking month = %q, want 2026-01", m)
	}

	if m := (domain.Statement{}).EarliestBookingMonth(); m != "" {
		t.Errorf("earliest booking month of no rows = %q, want none", m)
	}
}

// Of the rows split off, only those that are neither a certain repeat nor
// a lookalike of an earlier Imported transaction aren't in the books, and
// are counted per closed month, in order (spec story 13).
func TestTheDiagnosisCountsOnlyRowsNotInTheBooks(t *testing.T) {
	t.Parallel()

	file := closedJanuaryFile()
	file.Rows = append(file.Rows, statementRow(9, "2025-12-31", "-3.00", "T-9")) // new, in closed December

	got := file.WithoutClosedMonths("2026-02").ClosedMonthsNotInBooks(closedJanuaryEarlier())

	want := []domain.MonthRows{{Month: "2025-12", Rows: 1}, {Month: "2026-01", Rows: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("not in books = %+v, want %+v", got, want)
	}

	if got := closedJanuaryFile().WithoutClosedMonths("2026-01").ClosedMonthsNotInBooks(nil); len(got) != 0 {
		t.Errorf("not in books = %+v, want none while no month is closed", got)
	}
}

// A later import records how many rows it ignored in closed months and
// its earliest Booking month; a failed check carries the closed months'
// rows that aren't in the books (spec stories 12, 13).
func TestALaterImportCountsItsRowsInClosedMonths(t *testing.T) {
	t.Parallel()

	c := laterImport()
	c.RowsIgnoredInClosedMonths, c.EarliestBookingMonth = 3, "2026-02"
	c.ClosedMonthsNotInBooks = []domain.MonthRows{{Month: "2026-01", Rows: 1}}

	recorded := domain.StatementImportRecorded{
		ConnectionID: theConnection, AccountID: theAccount, FileHash: "def456", MappingVersion: 2, Currency: "EUR",
		ExportBalance: dec("2454.90"), RowsImported: 1, RowsSkipped: 2, RowsTotal: dec("-45.10"),
		BalanceCheck: domain.BalanceCheckPassed, RowsIgnoredInClosedMonths: 3, EarliestBookingMonth: "2026-02",
	}

	decidertest.Given(t, domain.StatementImport).When(c).Then(recorded)

	s := decidertest.Given(t, domain.StatementImport, domain.StatementImportEvent(recorded)).State()
	if s.RowsIgnoredInClosedMonths != 3 || s.EarliestBookingMonth != "2026-02" {
		t.Errorf("state = %+v, want 3 rows ignored in closed months and February earliest", s)
	}

	c.ExportBalance = dec("2449.90")

	_, err := domain.StatementImport.Decide(domain.StatementImport.Initial, c)

	failed, ok := errors.AsType[*domain.BalanceCheckFailedError](err)
	if !ok {
		t.Fatalf("error = %v, want a *BalanceCheckFailedError", err)
	}

	if !reflect.DeepEqual(failed.ClosedMonthsNotInBooks, c.ClosedMonthsNotInBooks) ||
		failed.RowsInClosedMonthsNotInBooks() != 1 {
		t.Errorf("failed check = %+v, want January's row not in the books", failed)
	}
}
