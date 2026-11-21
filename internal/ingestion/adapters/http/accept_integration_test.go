//go:build integration

package http_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type acceptedTransaction struct {
	importedTransaction

	TransactionID string `json:"transaction_id"`
}

type category struct {
	CategoryID string     `json:"category_id"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Children   []category `json:"children"`
}

// acceptFixture is an importFixture whose first Statement import is
// confirmed: an Opening balance of 510.50 and the pending rows Rent
// (2026-01-01, -1000.00), Salary (2026-01-15, 3000.00) and Coffee
// (2026-02-02, -10.50).
type acceptFixture struct {
	importFixture

	pending map[string]importedTransaction
}

func newAcceptFixture(t *testing.T, app *apitest.App, subject string) acceptFixture {
	t.Helper()

	f := newImportFixture(t, app, subject)
	f.confirm(t, statement, "2500.00")

	pending := map[string]importedTransaction{}
	for _, i := range f.imported(t, "?state=PENDING").Items {
		pending[i.Description] = i
	}

	return acceptFixture{importFixture: f, pending: pending}
}

func (f acceptFixture) acceptPath(description string) string {
	return "/api/households/" + f.household + "/imported-transactions/" + f.pending[description].ImportedTransactionID + "/accept"
}

// accept Accepts the pending row with the description; a nil body sends
// none at all.
func (f acceptFixture) accept(t *testing.T, description string, body map[string]any) *apitest.Response {
	t.Helper()

	if body == nil {
		return f.app.Do(t, http.MethodPost, f.acceptPath(description), f.token, nil)
	}

	return f.app.Post(t, f.acceptPath(description), f.token, body)
}

func (f acceptFixture) accepted(t *testing.T, description string, body map[string]any) acceptedTransaction {
	t.Helper()

	var a acceptedTransaction
	f.accept(t, description, body).AssertJSON(t, http.StatusOK, &a)

	return a
}

// categories returns the Household's Categories by name, at any depth.
func (f acceptFixture) categories(t *testing.T) map[string]category {
	t.Helper()

	var tree struct {
		Items []category `json:"items"`
	}

	f.app.Get(t, "/api/households/"+f.household+"/categories", f.token).AssertJSON(t, http.StatusOK, &tree)

	byName := map[string]category{}

	var walk func([]category)
	walk = func(cs []category) {
		for _, c := range cs {
			byName[c.Name] = c
			walk(c.Children)
		}
	}
	walk(tree.Items)

	return byName
}

func (f acceptFixture) createCategory(t *testing.T, name, kind string) string {
	t.Helper()

	var c category
	f.app.Post(t, "/api/households/"+f.household+"/categories", f.token, map[string]any{"name": name, "kind": kind}).
		AssertJSON(t, http.StatusCreated, &c)

	return c.CategoryID
}

// Accepting without a Category books the row's exact amount now, with its
// other side in Uncategorised (spec stories 46, 47, 51a).
func TestAcceptingWithoutACategoryBooksItToUncategorised(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	a := f.accepted(t, "Salary", map[string]any{})

	if a.State != "ACCEPTED" || a.TransactionID == "" || a.ImportedTransactionID != f.pending["Salary"].ImportedTransactionID {
		t.Errorf("accepted = %+v, want it ACCEPTED with its Transaction", a)
	}

	assertMoney(t, "accepted amount", a.Amount, "3000.00")

	if got := f.getAccount(t); got.Balance == nil {
		t.Fatal("the Account has no balance")
	} else {
		assertMoney(t, "balance", *got.Balance, "3510.50")
	}

	jan := f.transactions(t, "?booking_month=2026-01")
	if len(jan.Items) != 2 {
		t.Fatalf("January = %+v, want the Opening balance and the salary", jan.Items)
	}

	salary := jan.Items[1]
	if salary.TransactionID != a.TransactionID || salary.BookingDate != "2026-01-15" ||
		salary.TransactionDate != "2026-01-15" || salary.Description != "Salary" ||
		salary.CategoryName != "Uncategorised" || salary.CategoryID != f.categories(t)["Uncategorised"].CategoryID {
		t.Errorf("salary = %+v, want it booked to Uncategorised", salary)
	}

	assertMoney(t, "salary transaction", salary.Amount, "3000.00")

	pending := f.imported(t, "?state=PENDING")
	if len(pending.Items) != 2 {
		t.Errorf("pending = %+v, want Rent and Coffee", pending.Items)
	}

	all := f.imported(t, "")
	if len(all.Items) != 3 || all.Items[1].State != "ACCEPTED" {
		t.Errorf("imported = %+v, want the salary ACCEPTED", all.Items)
	}
}

// Accepting with a Category books the other side there; choosing
// Uncategorised is the same as choosing none (spec stories 45, 49).
func TestAcceptingWithACategoryBooksItThere(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	housing := f.createCategory(t, "Housing", "EXPENSE")
	uncategorised := f.categories(t)["Uncategorised"].CategoryID

	rent := f.accepted(t, "Rent", map[string]any{"category_id": housing})
	salary := f.accepted(t, "Salary", map[string]any{"category_id": uncategorised})

	if rent.State != "ACCEPTED" || salary.State != "ACCEPTED" {
		t.Errorf("rent = %+v, salary = %+v, want both ACCEPTED", rent, salary)
	}

	if got := f.getAccount(t); got.Balance == nil {
		t.Fatal("the Account has no balance")
	} else {
		assertMoney(t, "balance", *got.Balance, "2510.50")
	}

	jan := f.transactions(t, "?booking_month=2026-01")
	if len(jan.Items) != 3 {
		t.Fatalf("January = %+v, want the Opening balance, the rent and the salary", jan.Items)
	}

	// Both are booked on 2026-01-01: the Opening balance was recorded first.
	if r := jan.Items[1]; r.TransactionID != rent.TransactionID || r.CategoryID != housing || r.CategoryName != "Housing" ||
		r.Description != "Rent" || r.BookingDate != "2026-01-01" {
		t.Errorf("rent = %+v, want it booked to Housing", r)
	} else {
		assertMoney(t, "rent", r.Amount, "-1000.00")
	}

	if s := jan.Items[2]; s.TransactionID != salary.TransactionID || s.CategoryID != uncategorised ||
		s.CategoryName != "Uncategorised" {
		t.Errorf("salary = %+v, want it in Uncategorised", s)
	}

	if got := f.imported(t, "?state=ACCEPTED"); len(got.Items) != 2 || got.Items[0].Description != "Rent" ||
		got.Items[1].Description != "Salary" {
		t.Errorf("accepted = %+v, want Rent and Salary", got.Items)
	}
}

// An Imported transaction is booked once, and only to a Category of the
// Household that isn't Internal (Uncategorised aside). A refused Accept
// books nothing (spec stories 48, 49).
func TestAcceptingIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newAcceptFixture(t, app, "alice")
	bob := newAcceptFixture(t, app, "bob")

	cats := f.categories(t)
	bobsCategory := bob.createCategory(t, "Bob's", "EXPENSE")

	f.accept(t, "Rent", map[string]any{"category_id": cats["Opening balance"].CategoryID}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/internal-category")
	f.accept(t, "Rent", map[string]any{"category_id": cats["Household transfer"].CategoryID}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/internal-category")
	f.accept(t, "Rent", map[string]any{"category_id": bobsCategory}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-category")
	f.accept(t, "Rent", map[string]any{"category_id": "0198f0a0-0000-7000-8000-000000000999"}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-category")

	if got := f.imported(t, "?state=PENDING"); len(got.Items) != 3 {
		t.Errorf("pending = %+v, want all three still pending", got.Items)
	}

	if got := f.transactions(t, ""); len(got.Items) != 1 {
		t.Errorf("transactions = %+v, want only the Opening balance", got.Items)
	}

	first := f.accepted(t, "Rent", nil)

	f.accept(t, "Rent", nil).AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-already-accepted")

	if got := f.transactions(t, ""); len(got.Items) != 2 || got.Items[1].TransactionID != first.TransactionID {
		t.Errorf("transactions = %+v, want the Opening balance and the rent once", got.Items)
	}

	if got := f.getAccount(t); got.Balance == nil {
		t.Fatal("the Account has no balance")
	} else {
		assertMoney(t, "balance", *got.Balance, "-489.50")
	}

	f.app.Do(t, http.MethodPost, "/api/households/"+f.household+"/imported-transactions/0198f0a0-0000-7000-8000-000000000999/accept",
		f.token, nil).AssertProblem(t, http.StatusNotFound, "about:blank")
}

// Concurrent Accepts of one row book it once: the Imported transaction's
// version check makes the others retry and find it Accepted. Concurrent
// Accepts of different rows of one Account all book, retrying on the
// Account's version check (ADR-0011).
func TestConcurrentAcceptsBookEachRowOnce(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	const n = 8

	statuses := make([]int, n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			// Both in January, the Open month.
			description := []string{"Salary", "Rent"}[i%2]

			statuses[i] = f.accept(t, description, nil).Status
		})
	}

	wg.Wait()

	ok := 0

	for _, s := range statuses {
		switch s {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
		default:
			t.Errorf("status %d, want 200 or 409", s)
		}
	}

	if ok != 2 {
		t.Errorf("statuses = %v, want one 200 per row", statuses)
	}

	if got := f.transactions(t, ""); len(got.Items) != 3 {
		t.Errorf("transactions = %+v, want the Opening balance and each row once", got.Items)
	}

	if got := f.getAccount(t); got.Balance == nil {
		t.Fatal("the Account has no balance")
	} else {
		assertMoney(t, "balance", *got.Balance, "2510.50")
	}
}

// Retrying an Accept with its Idempotency-Key replays the result and never
// books twice (spec story 52).
func TestRetryingAnAcceptWithItsIdempotencyKeyBooksOnce(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	first := f.app.PostWithKey(t, f.acceptPath("Salary"), f.token, "accept-1", map[string]any{})
	if first.Status != http.StatusOK {
		t.Fatalf("first accept: %d %s", first.Status, first.Body)
	}

	retry := f.app.PostWithKey(t, f.acceptPath("Salary"), f.token, "accept-1", map[string]any{})
	if retry.Status != http.StatusOK || string(retry.Body) != string(first.Body) {
		t.Errorf("retry = %d %s, want the original %s", retry.Status, retry.Body, first.Body)
	}

	if got := f.transactions(t, ""); len(got.Items) != 2 {
		t.Errorf("transactions = %+v, want the Opening balance and the salary once", got.Items)
	}

	if got := f.getAccount(t); got.Balance == nil {
		t.Fatal("the Account has no balance")
	} else {
		assertMoney(t, "balance", *got.Balance, "3510.50")
	}
}

// Another Household can't Accept, nor see what an Accept booked.
func TestAnotherHouseholdCantAccept(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := newAcceptFixture(t, app, "alice")
	bob := newAcceptFixture(t, app, "bob")

	salary := alice.pending["Salary"].ImportedTransactionID

	// Bob through Alice's Household, and through his own with her IDs.
	for _, household := range []string{alice.household, bob.household} {
		app.Post(t, "/api/households/"+household+"/imported-transactions/"+salary+"/accept", bob.token, map[string]any{}).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	}

	if got := alice.imported(t, "?state=PENDING"); len(got.Items) != 3 {
		t.Errorf("Alice's pending = %+v, want all three", got.Items)
	}

	alice.accepted(t, "Salary", nil)

	for _, household := range []string{alice.household, bob.household} {
		path := "/api/households/" + household + "/accounts/" + alice.account
		app.Get(t, path+"/transactions", bob.token).AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Get(t, path+"/imported-transactions?state=ACCEPTED", bob.token).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	}

	if got := bob.transactions(t, ""); len(got.Items) != 1 {
		t.Errorf("Bob's transactions = %+v, want his Opening balance alone", got.Items)
	}
}

// With several Accepted, an Account's Transaction list pages through them
// in Booking date order, and a Booking month keeps its own (carried over
// from ticket 08).
func TestTheTransactionListPagesThroughAcceptedTransactions(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	// February's coffee is Accepted once January is closed.
	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Salary", "Rent")
	f.closed(t, "2026-01", "2510.50")
	f.accepted(t, "Coffee", nil)

	var (
		got    []string
		cursor string
		pages  int
	)

	for {
		query := "?limit=2"
		if cursor != "" {
			query += "&cursor=" + cursor
		}

		p := f.transactions(t, query)
		pages++

		if len(p.Items) > 2 {
			t.Fatalf("page %d has %d items, want at most 2", pages, len(p.Items))
		}

		for _, i := range p.Items {
			got = append(got, i.BookingDate+" "+i.Description)
		}

		if p.NextCursor == "" {
			break
		}

		cursor = p.NextCursor
	}

	want := []string{"2026-01-01 Opening balance", "2026-01-01 Rent", "2026-01-15 Salary", "2026-02-02 Coffee"}
	if pages != 2 || !sameJSON(t, got, want) {
		t.Errorf("walked %d pages: %v, want 2 pages: %v", pages, got, want)
	}

	jan := f.transactions(t, "?booking_month=2026-01&limit=2")
	if len(jan.Items) != 2 || jan.NextCursor == "" {
		t.Fatalf("January first page = %+v, want two and a cursor", jan)
	}

	rest := f.transactions(t, "?booking_month=2026-01&limit=2&cursor="+jan.NextCursor)
	if len(rest.Items) != 1 || rest.Items[0].Description != "Salary" || rest.NextCursor != "" {
		t.Errorf("January second page = %+v, want the salary alone", rest)
	}
}

// What an Accept recorded is rebuilt the same from the events.
func TestAcceptsSurviveARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newAcceptFixture(t, app, "alice")

	f.accepted(t, "Rent", map[string]any{"category_id": f.createCategory(t, "Housing", "EXPENSE")})
	f.accepted(t, "Salary", nil)

	before := []any{f.getAccount(t), f.transactions(t, ""), f.imported(t, "")}

	app.Rebuild(t)

	after := []any{f.getAccount(t), f.transactions(t, ""), f.imported(t, "")}
	if !sameJSON(t, after, before) {
		t.Errorf("after rebuild:\n  %+v\nbefore:\n  %+v", after, before)
	}

	// The rebuilt Imported transaction still knows it is Accepted.
	f.accept(t, "Salary", nil).AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-already-accepted")
}
