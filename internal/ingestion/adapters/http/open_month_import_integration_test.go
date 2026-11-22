//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// overlapAfterJanuary is a later file uploaded after January closed: its
// January rows are already in the books, and a new February row brings the
// bank's balance to 2510.50 - 10.50 - 20.00 = 2480.00.
const overlapAfterJanuary = "Date;Description;Amount\n" +
	"01/01/2026;Rent;-1.000,00\n" +
	"15/01/2026;Salary;3.000,00\n" +
	"10/02/2026;Groceries;-20,00\n"

// backDated is overlapAfterJanuary with a January row the books don't
// have: the bank's balance is 5.00 lower, at 2475.00.
const backDated = "Date;Description;Amount\n" +
	"15/01/2026;Salary;3.000,00\n" +
	"25/01/2026;Late fee;-5,00\n" +
	"10/02/2026;Groceries;-20,00\n"

// balanceCheckRefusal uploads the file expecting its Balance check to fail,
// and returns the problem.
func (f importFixture) balanceCheckRefusal(t *testing.T, file, balance string) (balanceCheckProblem, apitest.ProblemBody) {
	t.Helper()

	res := f.upload(t, "", file, balance, "")
	body := res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/balance-check-failed")

	var p balanceCheckProblem
	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatal(err)
	}

	return p, body
}

// A file overlapping closed January leaves January's rows out, counting
// them, so they are neither repeats nor Suspected duplicates; its new
// February row imports and the check passes (spec story 12).
func TestRowsInAClosedMonthAreIgnoredAndCounted(t *testing.T) {
	t.Parallel()
	f, _, _ := januaryClosed(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", overlapAfterJanuary, "2480.00", "").AssertJSON(t, http.StatusCreated, &si)

	if si.RowsImported != 1 || si.RowsIgnoredInClosedMonths != 2 || si.RowsSkippedAsCertainRepeats != 0 ||
		si.SuspectedDuplicates != 0 || si.BalanceCheck != "PASSED" {
		t.Errorf("statement import = %+v, want February's row imported and January's two ignored", si)
	}

	assertMoney(t, "rows total", si.RowsTotal, "-20.00")

	if got := f.importNamed(t, si.StatementImportID); got.RowsIgnoredInClosedMonths != 2 {
		t.Errorf("listed import = %+v, want its two ignored rows kept", got)
	}

	assertMonth(t, f.month(t, "2026-01"), "CLOSED", 0, 0, 0)
	assertMonth(t, f.month(t, "2026-02"), "OPEN", 2, 0, 0)
}

// A file with a back-dated row in closed January fails its check, which
// counts the row that isn't in the books and names the month to Reopen;
// the January rows the books have don't count (spec story 13).
func TestABackDatedRowInAClosedMonthIsDiagnosed(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	t.Run("one closed month", func(t *testing.T) {
		t.Parallel()
		f, _, _ := januaryClosed(t, app, "one")

		p, body := f.balanceCheckRefusal(t, backDated, "2475.00")

		if p.RowsInClosedMonthsNotInBooks == nil || *p.RowsInClosedMonthsNotInBooks != 1 {
			t.Errorf("problem = %+v, want one row in closed months not in the books", p)
		}

		assertMoney(t, "difference", *p.Difference, "-5.00")

		want := " Nothing was imported: check the file, its mapping and the Export balance. 1 row of this file " +
			"falls in closed January 2026 and isn't in the books. Reopen January 2026, then import the file again."
		if !strings.HasSuffix(body.Detail, want) {
			t.Errorf("detail = %q, want it to end %q", body.Detail, want)
		}

		if got := f.importList(t); len(got) != 1 {
			t.Errorf("imports = %+v, want only the first", got)
		}
	})

	t.Run("several closed months", func(t *testing.T) {
		t.Parallel()
		f := twoMonthsClosed(t, app, "several")

		_, body := f.balanceCheckRefusal(t, "Date;Description;Amount\n"+
			"20/01/2026;Late fee;-5,00\n"+
			"21/01/2026;Bonus;100,00\n"+
			"02/02/2026;Coffee;-10,50\n"+ // in the books
			"12/02/2026;Bus;-2,00\n", "2593.00")

		want := " 3 rows of this file fall in closed months and aren't in the books: 2 in January 2026 and 1 in " +
			"February 2026. Reopen February 2026, then January 2026, then import the file again."
		if !strings.HasSuffix(body.Detail, want) {
			t.Errorf("detail = %q, want it to end %q", body.Detail, want)
		}
	})

	t.Run("no row missing from the books", func(t *testing.T) {
		t.Parallel()
		f, _, _ := januaryClosed(t, app, "none")

		p, body := f.balanceCheckRefusal(t, overlapAfterJanuary, "2400.00")

		if p.RowsInClosedMonthsNotInBooks != nil {
			t.Errorf("problem = %+v, want no rows in closed months counted", p)
		}

		if !strings.HasSuffix(body.Detail, "the file, its mapping and the Export balance.") {
			t.Errorf("detail = %q, want no closed month named", body.Detail)
		}
	})
}

// Importing on 10 February while January is open: the February rows
// arrive pending in a month not open yet, and can't be Accepted or
// Discarded until it opens (spec story 10).
func TestRowsInAMonthNotOpenYetImportAsPending(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", "Date;Description;Amount\n05/01/2026;Groceries;-45,10\n05/02/2026;Bread;-2,00\n", "2452.90", "").
		AssertJSON(t, http.StatusCreated, &si)

	if si.RowsImported != 2 || si.RowsIgnoredInClosedMonths != 0 || si.BalanceCheck != "PASSED" {
		t.Errorf("statement import = %+v, want both rows imported", si)
	}

	assertMonth(t, f.month(t, "2026-01"), "OPEN", 3, 0, 0)
	assertMonth(t, f.month(t, "2026-02"), "NOT_OPEN_YET", 2, 0, 0)

	for _, i := range f.imported(t, "?state=PENDING").Items {
		f.pending[i.Description] = i
	}

	assertFebruaryNotOpenYet(t, f.accept(t, "Bread", nil))
	assertFebruaryNotOpenYet(t, f.discard(t, "Bread"))
}
