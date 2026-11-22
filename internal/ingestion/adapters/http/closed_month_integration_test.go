//go:build integration

package http_test

import (
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// januaryClosed is the acceptFixture with Rent and Salary Accepted into
// Housing and January closed at 2510.50: February is the Open month, and
// its coffee is still pending.
func januaryClosed(t *testing.T, app *apitest.App, subject string) (f acceptFixture, rent, salary acceptedTransaction) {
	t.Helper()

	f = newAcceptFixture(t, app, subject)
	housing := f.createCategory(t, "Housing", "EXPENSE")
	rent = f.accepted(t, "Rent", map[string]any{"category_id": housing})
	salary = f.accepted(t, "Salary", map[string]any{"category_id": housing})
	f.closed(t, "2026-01", "2510.50")

	return f, rent, salary
}

// assertMonthProblem checks a 409 about a month: its type, the spec's
// copy, and the months it names.
func assertMonthProblem(t *testing.T, res *apitest.Response, problemType, title, detail, month, open string) {
	t.Helper()

	p := monthConflict(t, res, problemType)
	if p.Title != title || p.Detail != detail || p.BookingMonth != month || p.OpenMonth != open {
		t.Errorf("problem = %+v, want %q / %q naming %s with %s open", p, title, detail, month, open)
	}
}

func assertMonthClosed(t *testing.T, res *apitest.Response) {
	t.Helper()

	assertMonthProblem(t, res, "/problems/month-closed", "Month closed",
		"January 2026 is closed. Reopen it to change its entries.", "2026-01", "2026-02")
}

func assertFebruaryNotOpenYet(t *testing.T, res *apitest.Response) {
	t.Helper()

	assertMonthProblem(t, res, "/problems/month-not-open-yet", "Month not open yet",
		"February 2026 isn't open yet. Close January 2026 first.", "2026-02", "2026-01")
}

// A closed month's Transactions can't be reversed, and can change
// Category, but never to Uncategorised (spec stories 5, 6).
func TestAClosedMonthsTransactionsAreLocked(t *testing.T) {
	t.Parallel()
	f, rent, _ := januaryClosed(t, apitest.Start(t), "alice")
	before := f.transactions(t, "")

	assertMonthClosed(t, f.reverse(t, rent.TransactionID))

	uncategorised := f.categories(t)["Uncategorised"].CategoryID
	p := f.setCategory(t, rent.TransactionID, uncategorised).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/uncategorised-in-closed-month")

	if p.Title != "Uncategorised in a closed month" || p.Detail != "January 2026 is closed, so its Transactions "+
		"can't go back to Uncategorised. Pick another Category." {
		t.Errorf("problem = %+v", p)
	}

	if after := f.transactions(t, ""); !sameJSON(t, after, before) {
		t.Errorf("transactions = %+v, want them untouched by the refusals: %+v", after.Items, before.Items)
	}

	var changed transaction
	f.setCategory(t, rent.TransactionID, f.createCategory(t, "Rent", "EXPENSE")).AssertJSON(t, http.StatusOK, &changed)

	if changed.CategoryName != "Rent" {
		t.Errorf("changed = %+v, want it moved to Rent", changed)
	}

	assertMonth(t, f.month(t, "2026-01"), "CLOSED", 0, 0, 0)
	f.assertBalance(t, "2510.50")
}

// A closed month's Imported transactions can't be Accepted or Discarded:
// the month comes before the row's own state (spec story 5). A later
// upload can't bring a pending or Suspected duplicate row into a closed
// month, since its rows there are ignored (spec story 12), so the
// Ingestion check for those is only a backstop.
func TestAClosedMonthsImportedTransactionsAreLocked(t *testing.T) {
	t.Parallel()
	f, _, _ := januaryClosed(t, apitest.Start(t), "alice")

	// Already Accepted, but the closed month is said first.
	assertMonthClosed(t, f.accept(t, "Rent", nil))
	assertMonthClosed(t, f.discard(t, "Rent"))

	if got := f.imported(t, "?state=ACCEPTED"); len(got.Items) != 2 {
		t.Errorf("accepted = %+v, want Rent and Salary untouched", got.Items)
	}

	f.assertBalance(t, "2510.50")
}

// A pending row booked in a month not open yet is visible, but can't be
// Accepted or Discarded until that month opens (spec story 10).
func TestAPendingRowInAMonthNotOpenYetWaits(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	assertFebruaryNotOpenYet(t, f.accept(t, "Coffee", nil))
	assertFebruaryNotOpenYet(t, f.discard(t, "Coffee"))

	assertMonth(t, f.month(t, "2026-02"), "NOT_OPEN_YET", 1, 0, 0)

	// Once January closes, February is open and the coffee can be Accepted.
	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")
	f.closed(t, "2026-01", "2510.50")

	if got := f.accepted(t, "Coffee", nil); got.State != "ACCEPTED" {
		t.Errorf("coffee = %+v, want it ACCEPTED", got)
	}
}

// A Suspected duplicate booked in a month not open yet can be kept or
// Discarded, so its import's waiting Balance check runs and the Open month
// can close (spec story 11).
func TestASuspectedDuplicateInAMonthNotOpenYetCanBeDecided(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	for _, tc := range []struct {
		action, state string
		// export makes the check pass for that decision: 510.50 - 1000.00 +
		// 3000.00 - 10.50 - 45.10 = 2454.90, and 10.50 less when the
		// coffee is kept as a second one.
		export string
	}{
		{"keep", "PENDING", "2444.40"},
		{"discard", "DISCARDED", "2454.90"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			t.Parallel()
			f := newSuspectedFixture(t, app, tc.action)

			var si statementImport
			f.upload(t, "", overlapWithoutIDs, tc.export, "").AssertJSON(t, http.StatusCreated, &si)

			suspected := f.suspected(t)
			f.decide(t, suspected["SALARY"].ImportedTransactionID, "discard", "DISCARDED")

			coffee := f.decide(t, suspected["coffee"].ImportedTransactionID, tc.action, tc.state)
			if coffee.BookingDate != "2026-02-02" {
				t.Fatalf("coffee = %+v, want it booked in February", coffee)
			}

			if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "PASSED" {
				t.Fatalf("check = %q, want it run and passed", got.BalanceCheck)
			}

			f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")
			assertMonth(t, f.closed(t, "2026-01", "2510.50"), "CLOSED", 0, 0, 0)
		})
	}
}

// A kept Suspected duplicate in a month not open yet is pending there, so
// a later Discard of it is refused like any pending row's.
func TestAKeptRowInAMonthNotOpenYetCantBeDiscarded(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")
	f.upload(t, "", overlapWithoutIDs, "2444.40", "").AssertJSON(t, http.StatusCreated, &statementImport{})

	coffee := f.suspected(t)["coffee"].ImportedTransactionID
	f.decide(t, coffee, "keep", "PENDING")

	res := f.app.Do(t, http.MethodPost, f.importedPath(coffee, "discard"), f.token, nil)
	assertFebruaryNotOpenYet(t, res)

	// Keeping it again says it is no longer a Suspected duplicate.
	f.app.Do(t, http.MethodPost, f.importedPath(coffee, "keep"), f.token, nil).
		AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-not-suspected-duplicate")
}
