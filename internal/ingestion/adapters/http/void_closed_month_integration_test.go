//go:build integration

package http_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// assertVoidBlockedBy checks the reasons a Void of the listed import would
// be refused.
func (f importFixture) assertVoidBlockedBy(t *testing.T, statementImportID string, want ...string) {
	t.Helper()

	if want == nil {
		want = []string{}
	}

	if got := f.importNamed(t, statementImportID).VoidBlockedBy; !reflect.DeepEqual(got, want) {
		t.Errorf("void blocked by = %v, want %v", got, want)
	}
}

// assertTouchesClosedJanuary checks the 409 of a Void touching closed
// January, with February open.
func assertTouchesClosedJanuary(t *testing.T, res *apitest.Response) {
	t.Helper()

	assertMonthProblem(t, res, "/problems/statement-import-touches-closed-month", "Statement import touches a closed month",
		"This import has rows in January 2026, which is closed. Reopen January 2026 first.", "2026-01", "2026-02")
}

// A later import with a row in January can't be voided once January
// closes, even though that row was Discarded: voiding never changes a
// closed month. Reopening January lets it be voided (spec stories 14, 15).
func TestVoidingALaterImportWithRowsInAClosedMonthIsRefused(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	var later statementImport
	f.upload(t, "", "Date;Description;Amount\n05/01/2026;Groceries;-45,10\n05/02/2026;Bread;-2,00\n", "2452.90", "").
		AssertJSON(t, http.StatusCreated, &later)

	if !reflect.DeepEqual(later.VoidBlockedBy, []string{}) {
		t.Errorf("uploaded import = %+v, want nothing blocking its Void", later)
	}

	for _, i := range f.imported(t, "?state=PENDING").Items {
		f.pending[i.Description] = i
	}

	f.discard(t, "Groceries").AssertJSON(t, http.StatusOK, &importedTransaction{})
	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent")
	f.acceptAll(t, f.createCategory(t, "Pay", "INCOME"), "Salary")
	f.closed(t, "2026-01", "2510.50")

	f.assertVoidBlockedBy(t, later.StatementImportID, "TOUCHES_CLOSED_MONTH")
	assertTouchesClosedJanuary(t, f.void(t, later.StatementImportID))

	if got := f.statesOf(t, later.StatementImportID); got["Bread"] != "PENDING" {
		t.Errorf("rows = %v, want the Bread still pending", got)
	}

	f.reopened(t, "2026-01")
	f.assertVoidBlockedBy(t, later.StatementImportID)

	if v := f.voided(t, later.StatementImportID); !reflect.DeepEqual(v.VoidBlockedBy, []string{}) {
		t.Errorf("voided import = %+v, want nothing blocking a voided import", v)
	}
}

// The first import touches the start date's month through its Opening
// balance, even with no rows there: once that month closes it can't be
// voided (spec story 14).
func TestVoidingTheFirstImportWithAClosedStartMonthIsRefused(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	// Only February's coffee: January holds just the Opening balance,
	// 2500.00 + 10.50.
	first := f.confirm(t, "Date;Description;Amount\n02/02/2026;Coffee;-10,50\n", "2500.00")
	f.assertVoidBlockedBy(t, first.StatementImportID)

	f.closed(t, "2026-01", "2510.50")

	f.assertVoidBlockedBy(t, first.StatementImportID, "TOUCHES_CLOSED_MONTH")
	assertTouchesClosedJanuary(t, f.void(t, first.StatementImportID))

	if a := f.getAccount(t); a.AwaitingFirstStatementImport {
		t.Errorf("account = %+v, want its Opening balance kept", a)
	}
}

// Rows a later import ignored in closed January don't block its Void,
// and void_blocked_by lists every reason the first import can't be
// voided at once (spec stories 12, 15).
func TestIgnoredRowsDontBlockVoid(t *testing.T) {
	t.Parallel()
	f, _, _ := januaryClosed(t, apitest.Start(t), "alice")

	var later statementImport
	f.upload(t, "", overlapAfterJanuary, "2480.00", "").AssertJSON(t, http.StatusCreated, &later)

	if later.RowsIgnoredInClosedMonths != 2 || !reflect.DeepEqual(later.VoidBlockedBy, []string{}) {
		t.Errorf("uploaded import = %+v, want its January rows ignored and nothing blocking its Void", later)
	}

	first := f.importList(t)[0].StatementImportID
	f.assertVoidBlockedBy(t, first, "NOT_LATEST", "HAS_ACCEPTED_ROWS", "TOUCHES_CLOSED_MONTH")
	f.assertVoidBlockedBy(t, later.StatementImportID)

	f.voided(t, later.StatementImportID)

	f.assertVoidBlockedBy(t, first, "HAS_ACCEPTED_ROWS", "TOUCHES_CLOSED_MONTH")
	f.assertVoidBlockedBy(t, later.StatementImportID)

	// The existing refusals still come first.
	f.void(t, first).AssertProblem(t, http.StatusConflict, "/problems/statement-import-has-accepted-rows")
}
