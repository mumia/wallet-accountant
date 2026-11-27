//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// directionProblem is a /problems/category-direction body.
type directionProblem struct {
	apitest.ProblemBody

	CategoryID   string `json:"category_id"`
	CategoryKind string `json:"category_kind"`
	Direction    string `json:"direction"`
}

// assertDirectionProblem checks a refused Category of the wrong kind: the
// Category it names, its kind and the amount's direction.
func assertDirectionProblem(t *testing.T, res *apitest.Response, category, kind, direction string) {
	t.Helper()

	res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/category-direction")

	var p directionProblem
	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatalf("problem = %s (%v)", res.Body, err)
	}

	if p.CategoryID != category || p.CategoryKind != kind || p.Direction != direction ||
		p.Title != "Category doesn't fit the amount" || p.Detail == "" {
		t.Errorf("problem = %+v, want %s %s on money %s", p, category, kind, direction)
	}
}

// An Expense Category takes only money out and an Income Category only
// money in; a refused Accept books nothing.
func TestAcceptingWithACategoryOfTheWrongKindIsRefused(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	housing := f.createCategory(t, "Housing", "EXPENSE")
	pay := f.createCategory(t, "Pay", "INCOME")

	assertDirectionProblem(t, f.accept(t, "Salary", map[string]any{"category_id": housing}), housing, "EXPENSE", "IN")
	assertDirectionProblem(t, f.accept(t, "Rent", map[string]any{"category_id": pay}), pay, "INCOME", "OUT")

	if got := f.imported(t, "?state=PENDING"); len(got.Items) != 3 {
		t.Errorf("pending = %+v, want all three still pending", got.Items)
	}

	if got := f.transactions(t, ""); len(got.Items) != 1 {
		t.Errorf("transactions = %+v, want the Opening balance only", got.Items)
	}

	// February isn't open yet: the month refusal comes first, as before.
	f.accept(t, "Coffee", map[string]any{"category_id": pay}).
		AssertProblem(t, http.StatusConflict, "/problems/month-not-open-yet")

	// Each kind fits its own direction, and Uncategorised fits either.
	f.accepted(t, "Salary", map[string]any{"category_id": pay})
	f.accepted(t, "Rent", map[string]any{"category_id": f.categories(t)["Uncategorised"].CategoryID})
}

// A zero amount has no direction, so it takes either kind.
func TestAZeroRowTakesEitherKind(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")
	f.confirm(t, "Date;Description;Amount\n"+
		"10/01/2026;Waived fee;0,00\n"+
		"11/01/2026;Waived interest;0,00\n", "100.00")

	a := acceptFixture{importFixture: f, pending: map[string]importedTransaction{}}
	for _, i := range f.imported(t, "?state=PENDING").Items {
		a.pending[i.Description] = i
	}

	fee := a.accepted(t, "Waived fee", map[string]any{"category_id": a.createCategory(t, "Fees", "EXPENSE")})
	a.accepted(t, "Waived interest", map[string]any{"category_id": a.createCategory(t, "Interest", "INCOME")})

	a.setCategory(t, fee.TransactionID, a.createCategory(t, "Refunds", "INCOME")).AssertJSON(t, http.StatusOK, &transaction{})
}

// Setting a Category follows the same rule, in a closed month too, but a
// Transaction that can't take any Category says so first.
func TestSettingACategoryOfTheWrongKindIsRefused(t *testing.T) {
	t.Parallel()
	f, rent, salary := januaryClosed(t, apitest.Start(t), "alice")
	groceries := f.createCategory(t, "Groceries", "EXPENSE")
	bonus := f.createCategory(t, "Bonus", "INCOME")
	before := f.transactions(t, "")

	// January is closed.
	assertDirectionProblem(t, f.setCategory(t, salary.TransactionID, groceries), groceries, "EXPENSE", "IN")
	assertDirectionProblem(t, f.setCategory(t, rent.TransactionID, bonus), bonus, "INCOME", "OUT")

	if after := f.transactions(t, ""); !sameJSON(t, after, before) {
		t.Errorf("transactions = %+v, want them untouched by the refusals: %+v", after.Items, before.Items)
	}

	f.setCategory(t, salary.TransactionID, bonus).AssertJSON(t, http.StatusOK, &transaction{})
	f.setCategory(t, rent.TransactionID, groceries).AssertJSON(t, http.StatusOK, &transaction{})
}

// A reversal and a reversed Transaction keep their Category whatever its
// kind: their own refusal comes before the direction's.
func TestAReversalsRefusalComesBeforeTheDirection(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	rent := f.accepted(t, "Rent", map[string]any{"category_id": f.createCategory(t, "Housing", "EXPENSE")})
	r := f.reversed(t, rent.TransactionID)
	pay := f.createCategory(t, "Pay", "INCOME")

	// Pay on Rent, which is money out, is the wrong kind; so is Housing on
	// its reversal, which is money in.
	f.setCategory(t, rent.TransactionID, pay).
		AssertProblem(t, http.StatusConflict, "/problems/transaction-reversed")
	f.setCategory(t, r.TransactionID, f.createCategory(t, "Rent", "EXPENSE")).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/transaction-is-a-reversal")
}
