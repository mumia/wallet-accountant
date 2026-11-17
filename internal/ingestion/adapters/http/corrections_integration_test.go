//go:build integration

package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// transaction is a Transaction as changeTransaction and reverseTransaction
// return it.
type transaction struct {
	TransactionID           string `json:"transaction_id"`
	AccountID               string `json:"account_id"`
	Amount                  money  `json:"amount"`
	BookingDate             string `json:"booking_date"`
	TransactionDate         string `json:"transaction_date"`
	ValueDate               string `json:"value_date"`
	Description             string `json:"description"`
	Payee                   string `json:"payee"`
	CategoryID              string `json:"category_id"`
	CategoryName            string `json:"category_name"`
	ImportedTransactionID   string `json:"imported_transaction_id"`
	ReversalOfTransactionID string `json:"reversal_of_transaction_id"`
	ReversedByTransactionID string `json:"reversed_by_transaction_id"`
}

// listedTransaction is a line of an Account's Transaction list with its
// reversal links.
type listedTransaction struct {
	accountTransaction

	ReversalOfTransactionID string `json:"reversal_of_transaction_id"`
	ReversedByTransactionID string `json:"reversed_by_transaction_id"`
}

func (f acceptFixture) transactionPath(id string) string {
	return "/households/" + f.household + "/transactions/" + id
}

func (f acceptFixture) setCategory(t *testing.T, transactionID, categoryID string) *apitest.Response {
	t.Helper()

	raw, err := json.Marshal(map[string]any{"category_id": categoryID})
	if err != nil {
		t.Fatal(err)
	}

	return f.app.Do(t, http.MethodPatch, f.transactionPath(transactionID), f.token, bytes.NewReader(raw))
}

func (f acceptFixture) reverse(t *testing.T, transactionID string) *apitest.Response {
	t.Helper()

	return f.app.Do(t, http.MethodPost, f.transactionPath(transactionID)+"/reverse", f.token, nil)
}

func (f acceptFixture) reversed(t *testing.T, transactionID string) transaction {
	t.Helper()

	var r transaction
	f.reverse(t, transactionID).AssertJSON(t, http.StatusCreated, &r)

	return r
}

func (f acceptFixture) discard(t *testing.T, description string) *apitest.Response {
	t.Helper()

	return f.app.Do(t, http.MethodPost,
		"/households/"+f.household+"/imported-transactions/"+f.pending[description].ImportedTransactionID+"/discard",
		f.token, nil)
}

func (f acceptFixture) listed(t *testing.T, query string) []listedTransaction {
	t.Helper()

	var p page[listedTransaction]
	f.app.Get(t, f.accountPath()+"/transactions"+query, f.token).AssertJSON(t, http.StatusOK, &p)

	return p.Items
}

// openingBalanceTransaction is the Account's Opening balance Transaction.
func (f acceptFixture) openingBalanceTransaction(t *testing.T) string {
	t.Helper()

	return f.transactions(t, "").Items[0].TransactionID
}

func (f acceptFixture) assertBalance(t *testing.T, want string) {
	t.Helper()

	if got := f.getAccount(t); got.Balance == nil {
		t.Fatal("the Account has no balance")
	} else {
		assertMoney(t, "balance", *got.Balance, want)
	}
}

// A Transaction Accepted into Uncategorised gets its Category later, and a
// wrong Category can be corrected, back to Uncategorised too. Its amount
// and the Account's balance never change (spec stories 50, 51, 51a).
func TestRecategorisingATransaction(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	wages := f.createCategory(t, "Wages", "INCOME")
	uncategorised := f.categories(t)["Uncategorised"].CategoryID
	salary := f.accepted(t, "Salary", nil)

	var got transaction
	f.setCategory(t, salary.TransactionID, wages).AssertJSON(t, http.StatusOK, &got)

	if got.TransactionID != salary.TransactionID || got.CategoryID != wages || got.CategoryName != "Wages" ||
		got.AccountID != f.account || got.BookingDate != "2026-01-15" || got.Description != "Salary" ||
		got.ImportedTransactionID != salary.ImportedTransactionID || got.ReversedByTransactionID != "" {
		t.Errorf("recategorised = %+v, want the salary in Wages", got)
	}

	assertMoney(t, "recategorised amount", got.Amount, "3000.00")

	jan := f.transactions(t, "?booking_month=2026-01")
	if len(jan.Items) != 2 || jan.Items[1].CategoryID != wages || jan.Items[1].CategoryName != "Wages" {
		t.Fatalf("January = %+v, want the salary listed in Wages", jan.Items)
	}

	assertMoney(t, "listed salary", jan.Items[1].Amount, "3000.00")
	f.assertBalance(t, "3510.50")

	// Setting the Category it already has changes nothing.
	f.setCategory(t, salary.TransactionID, wages).AssertJSON(t, http.StatusOK, &got)

	if got.CategoryID != wages {
		t.Errorf("unchanged = %+v, want it still in Wages", got)
	}

	f.setCategory(t, salary.TransactionID, uncategorised).AssertJSON(t, http.StatusOK, &got)

	if got.CategoryID != uncategorised || got.CategoryName != "Uncategorised" {
		t.Errorf("moved back = %+v, want it in Uncategorised", got)
	}

	if l := f.transactions(t, "?booking_month=2026-01").Items[1]; l.CategoryID != uncategorised {
		t.Errorf("listed = %+v, want it in Uncategorised again", l)
	}

	f.assertBalance(t, "3510.50")
}

// Reversing an Accepted Transaction records a linked reversing Transaction
// beside it, moves the balance back, and returns the Imported transaction
// to pending, to be Accepted again (spec stories 51b, 51c, 51d).
func TestReversingAnAcceptedTransactionThenAcceptingAgain(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	housing := f.createCategory(t, "Housing", "EXPENSE")
	rent := f.accepted(t, "Rent", map[string]any{"category_id": housing})
	f.assertBalance(t, "-489.50")

	r := f.reversed(t, rent.TransactionID)

	if r.TransactionID == "" || r.TransactionID == rent.TransactionID || r.ReversalOfTransactionID != rent.TransactionID ||
		r.BookingDate != "2026-01-01" || r.TransactionDate != "2026-01-01" || r.Description != "Rent" ||
		r.CategoryID != housing || r.AccountID != f.account || r.ImportedTransactionID != "" {
		t.Errorf("reversal = %+v, want it linked to the rent, on its Booking date, in Housing", r)
	}

	assertMoney(t, "reversal amount", r.Amount, "1000.00")
	f.assertBalance(t, "510.50")

	jan := f.listed(t, "?booking_month=2026-01")
	if len(jan) != 3 {
		t.Fatalf("January = %+v, want the Opening balance, the rent and its reversal", jan)
	}

	byID := map[string]listedTransaction{}
	for _, l := range jan {
		byID[l.TransactionID] = l
	}

	if l := byID[rent.TransactionID]; l.ReversedByTransactionID != r.TransactionID || l.ReversalOfTransactionID != "" {
		t.Errorf("listed rent = %+v, want it reversed by %s", l, r.TransactionID)
	} else {
		assertMoney(t, "listed rent", l.Amount, "-1000.00")
	}

	if l := byID[r.TransactionID]; l.ReversalOfTransactionID != rent.TransactionID || l.CategoryID != housing {
		t.Errorf("listed reversal = %+v, want it reversing %s", l, rent.TransactionID)
	} else {
		assertMoney(t, "listed reversal", l.Amount, "1000.00")
	}

	pending := f.imported(t, "?state=PENDING")
	if len(pending.Items) != 3 || pending.Items[0].Description != "Rent" {
		t.Fatalf("pending = %+v, want the rent pending again", pending.Items)
	}

	var raw map[string]any
	f.app.Get(t, f.accountPath()+"/imported-transactions?state=PENDING", f.token).AssertJSON(t, http.StatusOK, &raw)

	if first := raw["items"].([]any)[0].(map[string]any); first["transaction_id"] != nil {
		t.Errorf("pending rent = %v, want it no longer linked to a Transaction", first)
	}

	again := f.accepted(t, "Rent", nil)
	if again.TransactionID == rent.TransactionID || again.State != "ACCEPTED" {
		t.Errorf("accepted again = %+v, want a new Transaction", again)
	}

	f.assertBalance(t, "-489.50")

	if got := f.transactions(t, "?booking_month=2026-01"); len(got.Items) != 4 ||
		got.Items[3].TransactionID != again.TransactionID || got.Items[3].CategoryName != "Uncategorised" {
		t.Errorf("January = %+v, want the rent booked again in Uncategorised", got.Items)
	}
}

// A reversed Transaction's Imported transaction can be Discarded instead,
// and Discarded is final (spec stories 51d, 51e).
func TestReversingThenDiscarding(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	coffee := f.accepted(t, "Coffee", nil)

	f.discard(t, "Coffee").AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-already-accepted")

	f.reversed(t, coffee.TransactionID)

	var d importedTransaction
	f.discard(t, "Coffee").AssertJSON(t, http.StatusOK, &d)

	if d.State != "DISCARDED" || d.ImportedTransactionID != f.pending["Coffee"].ImportedTransactionID {
		t.Errorf("discarded = %+v, want the coffee DISCARDED", d)
	}

	if got := f.imported(t, "?state=DISCARDED"); len(got.Items) != 1 || got.Items[0].Description != "Coffee" {
		t.Errorf("discarded = %+v, want the coffee", got.Items)
	}

	if got := f.imported(t, "?state=PENDING"); len(got.Items) != 2 {
		t.Errorf("pending = %+v, want Rent and Salary", got.Items)
	}

	f.assertBalance(t, "510.50")

	f.accept(t, "Coffee", nil).AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-discarded")
	f.discard(t, "Coffee").AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-already-discarded")

	// A pending row is Discarded without ever being Accepted.
	f.discard(t, "Salary").AssertJSON(t, http.StatusOK, &d)

	if d.State != "DISCARDED" {
		t.Errorf("discarded salary = %+v", d)
	}

	f.app.Do(t, http.MethodPost, "/households/"+f.household+"/imported-transactions/0198f0a0-0000-7000-8000-000000000999/discard",
		f.token, nil).AssertProblem(t, http.StatusNotFound, "about:blank")
}

// A Discarded row no longer counts in a later import's Balance check nor
// as a certain repeat, so a later file brings it in again (ADR-0018).
func TestADiscardedRowIsLeftOutOfLaterBalanceChecks(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newLaterImportFixture(t, app, "alice")

	var coffee string

	for _, i := range f.imported(t, "?state=PENDING").Items {
		if i.Description == "Coffee" {
			coffee = i.ImportedTransactionID
		}
	}

	app.Do(t, http.MethodPost, "/households/"+f.household+"/imported-transactions/"+coffee+"/discard", f.token, nil).
		AssertJSON(t, http.StatusOK, &importedTransaction{})

	// 510.50 - 1000.00 + 3000.00 (the coffee Discarded) + the file's coffee
	// and groceries (-10.50 - 45.10) = 2454.90.
	var si statementImport
	f.upload(t, "", overlapping, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	if si.RowsImported != 2 || si.RowsSkippedAsCertainRepeats != 1 || si.BalanceCheck != "PASSED" {
		t.Errorf("statement import = %+v, want the coffee imported again, the salary skipped, the check passed", si)
	}
}

// The Opening balance Transaction changes only by voiding, and corrections
// can't pile up: reversing it, a reversal or a reversed Transaction is
// refused, and so is recategorising any of them (spec story 51f). A refusal
// changes nothing.
func TestCorrectionsAreRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newAcceptFixture(t, app, "alice")
	bob := newAcceptFixture(t, app, "bob")

	cats := f.categories(t)
	groceries := f.createCategory(t, "Groceries", "EXPENSE")
	opening := f.openingBalanceTransaction(t)
	coffee := f.accepted(t, "Coffee", nil)
	salary := f.accepted(t, "Salary", nil)

	f.setCategory(t, salary.TransactionID, cats["Opening balance"].CategoryID).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/internal-category")
	f.setCategory(t, salary.TransactionID, cats["Household transfer"].CategoryID).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/internal-category")
	f.setCategory(t, salary.TransactionID, bob.createCategory(t, "Bob's", "INCOME")).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-category")
	f.setCategory(t, salary.TransactionID, "0198f0a0-0000-7000-8000-000000000999").
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-category")

	f.reverse(t, opening).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/opening-balance-transaction")
	f.setCategory(t, opening, groceries).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/opening-balance-transaction")

	r := f.reversed(t, coffee.TransactionID)

	f.reverse(t, coffee.TransactionID).AssertProblem(t, http.StatusConflict, "/problems/transaction-already-reversed")
	f.reverse(t, r.TransactionID).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/transaction-is-a-reversal")
	f.setCategory(t, coffee.TransactionID, groceries).
		AssertProblem(t, http.StatusConflict, "/problems/transaction-reversed")
	f.setCategory(t, r.TransactionID, groceries).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/transaction-is-a-reversal")

	unknown := "0198f0a0-0000-7000-8000-000000000999"
	f.reverse(t, unknown).AssertProblem(t, http.StatusNotFound, "about:blank")
	f.setCategory(t, unknown, groceries).AssertProblem(t, http.StatusNotFound, "about:blank")

	listed := f.listed(t, "")
	if len(listed) != 4 {
		t.Fatalf("transactions = %+v, want the Opening balance, the salary, the coffee and its reversal", listed)
	}

	for _, l := range listed {
		if l.TransactionID == salary.TransactionID && l.CategoryName != "Uncategorised" {
			t.Errorf("salary = %+v, want it still in Uncategorised", l)
		}

		if l.TransactionID == opening && l.CategoryName != "Opening balance" {
			t.Errorf("opening balance = %+v, want it unchanged", l)
		}
	}

	f.assertBalance(t, "3510.50")
}

// Retrying a reversal with its Idempotency-Key replays it and reverses once.
func TestRetryingAReversalWithItsIdempotencyKeyReversesOnce(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	salary := f.accepted(t, "Salary", nil)
	path := f.transactionPath(salary.TransactionID) + "/reverse"

	first := f.app.DoWithHeader(t, http.MethodPost, path, f.token, http.Header{"Idempotency-Key": {"reverse-1"}}, nil)
	if first.Status != http.StatusCreated {
		t.Fatalf("first reverse: %d %s", first.Status, first.Body)
	}

	retry := f.app.DoWithHeader(t, http.MethodPost, path, f.token, http.Header{"Idempotency-Key": {"reverse-1"}}, nil)
	if retry.Status != http.StatusCreated || string(retry.Body) != string(first.Body) {
		t.Errorf("retry = %d %s, want the original %s", retry.Status, retry.Body, first.Body)
	}

	if got := f.transactions(t, ""); len(got.Items) != 3 {
		t.Errorf("transactions = %+v, want the Opening balance, the salary and one reversal", got.Items)
	}

	f.assertBalance(t, "510.50")
}

// Another Household can't recategorise, reverse or Discard.
func TestAnotherHouseholdCantCorrect(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := newAcceptFixture(t, app, "alice")
	bob := newAcceptFixture(t, app, "bob")

	salary := alice.accepted(t, "Salary", nil)
	bobsCategory := bob.createCategory(t, "Wages", "INCOME")
	coffee := alice.pending["Coffee"].ImportedTransactionID

	raw, err := json.Marshal(map[string]any{"category_id": bobsCategory})
	if err != nil {
		t.Fatal(err)
	}

	// Bob through Alice's Household, and through his own with her IDs.
	for _, household := range []string{alice.household, bob.household} {
		path := "/households/" + household + "/transactions/" + salary.TransactionID
		app.Do(t, http.MethodPatch, path, bob.token, bytes.NewReader(raw)).
			AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Do(t, http.MethodPost, path+"/reverse", bob.token, nil).AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Do(t, http.MethodPost, "/households/"+household+"/imported-transactions/"+coffee+"/discard", bob.token, nil).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	}

	if got := alice.transactions(t, ""); len(got.Items) != 2 || got.Items[1].CategoryName != "Uncategorised" {
		t.Errorf("Alice's transactions = %+v, want the salary untouched", got.Items)
	}

	if got := alice.imported(t, "?state=PENDING"); len(got.Items) != 2 {
		t.Errorf("Alice's pending = %+v, want Rent and Coffee", got.Items)
	}
}

// What corrections recorded is rebuilt the same from the events.
func TestCorrectionsSurviveARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newAcceptFixture(t, app, "alice")

	salary := f.accepted(t, "Salary", nil)
	f.setCategory(t, salary.TransactionID, f.createCategory(t, "Wages", "INCOME")).AssertJSON(t, http.StatusOK, &transaction{})

	rent := f.accepted(t, "Rent", nil)
	f.reversed(t, rent.TransactionID)
	f.discard(t, "Rent").AssertJSON(t, http.StatusOK, &importedTransaction{})

	before := []any{f.getAccount(t), f.listed(t, ""), f.imported(t, "")}

	app.Rebuild(t)

	after := []any{f.getAccount(t), f.listed(t, ""), f.imported(t, "")}
	if !sameJSON(t, after, before) {
		t.Errorf("after rebuild:\n  %+v\nbefore:\n  %+v", after, before)
	}

	f.reverse(t, rent.TransactionID).AssertProblem(t, http.StatusConflict, "/problems/transaction-already-reversed")
	f.discard(t, "Rent").AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-already-discarded")
}
