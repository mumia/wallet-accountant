//go:build integration

package http_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// januaryBonus is a later file with one new January row: 510.50 + 1989.50
// + 100.00 = 2600.00.
const januaryBonus = "Date;Description;Amount\n20/01/2026;Bonus;100,00\n"

// closedJanuary is an acceptFixture whose Rent and Salary are Accepted
// into Housing and whose January is closed at 2510.50.
type closedJanuary struct {
	acceptFixture

	housing string
	rent    acceptedTransaction
	salary  acceptedTransaction
}

func newClosedJanuary(t *testing.T, app *apitest.App) closedJanuary {
	t.Helper()

	f := newAcceptFixture(t, app, "alice")
	housing := f.createCategory(t, "Housing", "EXPENSE")
	rent := f.accepted(t, "Rent", map[string]any{"category_id": housing})
	salary := f.accepted(t, "Salary", map[string]any{"category_id": housing})
	assertMonth(t, f.closed(t, "2026-01", "2510.50"), "CLOSED", 0, 0, 0)

	return closedJanuary{acceptFixture: f, housing: housing, rent: rent, salary: salary}
}

// uploadBonus uploads januaryBonus as a later import and makes its row
// Acceptable by description.
func (f closedJanuary) uploadBonus(t *testing.T) statementImport {
	t.Helper()

	var si statementImport
	f.upload(t, "", januaryBonus, "2600.00", "").AssertJSON(t, http.StatusCreated, &si)

	for _, i := range f.imported(t, "?state=PENDING").Items {
		f.pending[i.Description] = i
	}

	return si
}

// assertFlagged checks that the month is closed and flagged, with its
// Month close kept.
func assertFlagged(t *testing.T, got accountMonth, statement string) {
	t.Helper()

	if got.State != "CLOSED" || !got.Flagged || got.StatementBalance == nil || got.ClosedAt == "" {
		t.Fatalf("%s = %+v, want it closed and flagged", got.BookingMonth, got)
	}

	assertMoney(t, "statement balance", *got.StatementBalance, statement)
}

// A closed month can still be corrected: an Accept, a Category change and
// a reversal in it are allowed, and each flags it. A change in another
// month leaves it unflagged (spec story 62).
func TestAChangeInAClosedMonthFlagsIt(t *testing.T) {
	t.Parallel()

	t.Run("a change in an open month", func(t *testing.T) {
		t.Parallel()
		f := newClosedJanuary(t, apitest.Start(t))

		f.accepted(t, "Coffee", nil)

		assertMonth(t, f.month(t, "2026-01"), "CLOSED", 0, 0, 0)
		assertMonth(t, f.month(t, "2026-02"), "OPEN", 0, 0, 1)
	})

	t.Run("accept", func(t *testing.T) {
		t.Parallel()
		f := newClosedJanuary(t, apitest.Start(t))
		f.uploadBonus(t)

		f.accepted(t, "Bonus", map[string]any{"category_id": f.housing})

		january := f.month(t, "2026-01")
		assertFlagged(t, january, "2510.50")
		assertMoney(t, "computed balance", *january.ComputedBalance, "2610.50")
		assertMonth(t, f.month(t, "2026-02"), "OPEN", 1, 0, 0)
	})

	t.Run("recategorise", func(t *testing.T) {
		t.Parallel()
		f := newClosedJanuary(t, apitest.Start(t))

		f.setCategory(t, f.rent.TransactionID, f.createCategory(t, "Rent", "EXPENSE")).
			AssertJSON(t, http.StatusOK, &transaction{})

		january := f.month(t, "2026-01")
		assertFlagged(t, january, "2510.50")

		if january.Uncategorised != 0 || january.Pending != 0 {
			t.Errorf("january = %+v, want nothing left to do", january)
		}
	})

	t.Run("recategorise to Uncategorised", func(t *testing.T) {
		t.Parallel()
		f := newClosedJanuary(t, apitest.Start(t))

		f.setCategory(t, f.salary.TransactionID, f.categories(t)["Uncategorised"].CategoryID).
			AssertJSON(t, http.StatusOK, &transaction{})

		january := f.month(t, "2026-01")
		assertFlagged(t, january, "2510.50")

		if january.Uncategorised != 1 {
			t.Errorf("january = %+v, want one Uncategorised", january)
		}
	})

	t.Run("reverse", func(t *testing.T) {
		t.Parallel()
		f := newClosedJanuary(t, apitest.Start(t))

		f.reversed(t, f.rent.TransactionID)

		january := f.month(t, "2026-01")
		assertFlagged(t, january, "2510.50")
		assertMoney(t, "computed balance", *january.ComputedBalance, "3510.50")

		// The Imported transaction is back to pending in the closed month.
		if january.Pending != 1 {
			t.Errorf("january = %+v, want the reversed row pending again", january)
		}
	})
}

// A change that moves a closed month's balance moves the closing balance of
// every later month, so every later closed month is flagged too; a
// recategorisation moves no balance and flags only its own month.
func TestABalanceChangeFlagsEveryLaterClosedMonth(t *testing.T) {
	t.Parallel()

	closedFebruary := func(t *testing.T) closedJanuary {
		t.Helper()

		f := newClosedJanuary(t, apitest.Start(t))
		f.accepted(t, "Coffee", map[string]any{"category_id": f.housing})
		assertMonth(t, f.closed(t, "2026-02", "2500.00"), "CLOSED", 0, 0, 0)

		return f
	}

	t.Run("reverse", func(t *testing.T) {
		t.Parallel()
		f := closedFebruary(t)

		f.reversed(t, f.rent.TransactionID)

		assertFlagged(t, f.month(t, "2026-01"), "2510.50")
		february := f.month(t, "2026-02")
		assertFlagged(t, february, "2500.00")
		assertMoney(t, "computed balance", *february.ComputedBalance, "3500.00")
	})

	t.Run("recategorise", func(t *testing.T) {
		t.Parallel()
		f := closedFebruary(t)

		f.setCategory(t, f.rent.TransactionID, f.createCategory(t, "Rent", "EXPENSE")).
			AssertJSON(t, http.StatusOK, &transaction{})

		assertFlagged(t, f.month(t, "2026-01"), "2510.50")

		if february := f.month(t, "2026-02"); february.Flagged {
			t.Errorf("february = %+v, want it unflagged: a recategorisation moves no balance", february)
		}
	})
}

// A later import bringing rows into a closed month shows them as pending,
// counted in its status; the month isn't flagged until something changes
// it (spec story 63).
func TestNewRowsInAClosedMonthArePending(t *testing.T) {
	t.Parallel()
	f := newClosedJanuary(t, apitest.Start(t))

	si := f.uploadBonus(t)
	if si.BalanceCheck != "PASSED" || si.RowsImported != 1 {
		t.Fatalf("import = %+v, want one row and the check passed", si)
	}

	january := f.month(t, "2026-01")
	if january.State != "CLOSED" || january.Pending != 1 || january.Flagged {
		t.Errorf("january = %+v, want it closed with one pending, not yet flagged", january)
	}

	if got := f.imported(t, "?state=PENDING&booking_month=2026-01").Items; len(got) != 1 || got[0].Description != "Bonus" {
		t.Errorf("pending in january = %+v, want the bonus", got)
	}
}

// Voiding an import whose rows were in a closed month flags it (spec
// story 62).
func TestVoidingAnImportWithRowsInAClosedMonthFlagsIt(t *testing.T) {
	t.Parallel()
	f := newClosedJanuary(t, apitest.Start(t))
	si := f.uploadBonus(t)

	f.voided(t, si.StatementImportID)

	january := f.month(t, "2026-01")
	assertFlagged(t, january, "2510.50")

	if january.Pending != 0 {
		t.Errorf("january = %+v, want the bonus Discarded", january)
	}

	// February had none of the voided import's rows.
	assertMonth(t, f.month(t, "2026-02"), "OPEN", 1, 0, 0)
}

// Voiding the first import reverses the Opening balance on the start date,
// which flags a closed start month (spec story 62).
func TestVoidingTheFirstImportFlagsAClosedStartMonth(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	for _, d := range []string{"Rent", "Salary"} {
		f.discard(t, d).AssertJSON(t, http.StatusOK, &importedTransaction{})
	}

	assertMonth(t, f.closed(t, "2026-01", "510.50"), "CLOSED", 0, 0, 0)

	first := f.importList(t)[0]
	f.voided(t, first.StatementImportID)

	january := f.month(t, "2026-01")
	assertFlagged(t, january, "510.50")

	if january.ComputedBalance != nil {
		t.Errorf("january = %+v, want no computed balance while awaiting the first import", january)
	}

	assertMonth(t, f.month(t, "2026-02"), "OPEN", 0, 0, 0)
}

// Flags are rebuilt the same from the events.
func TestFlagsSurviveARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newClosedJanuary(t, app)
	si := f.uploadBonus(t)
	f.voided(t, si.StatementImportID)

	before := f.month(t, "2026-01")

	app.Rebuild(t)

	if after := f.month(t, "2026-01"); !reflect.DeepEqual(after, before) || !after.Flagged {
		t.Errorf("after rebuild %+v, before %+v", after, before)
	}
}
