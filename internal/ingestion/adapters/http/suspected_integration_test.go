//go:build integration

package http_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// overlapWithoutIDs follows statement (no bank transaction IDs): its
// salary and coffee only look like the first file's, so they are Suspected
// duplicates; the groceries are new. If both are repeats the bank's balance
// is 2500.00 - 45.10 = 2454.90; if the coffee is a second, real one, it is
// 2444.40.
const overlapWithoutIDs = "Date;Description;Amount\n" +
	"15/01/2026;SALARY;3.000,00\n" +
	"02/02/2026;  coffee ;-10,50\n" +
	"10/02/2026;Groceries;-45,10\n"

// suspectedFixture is an Account whose first import was statement, with
// its pending rows by description.
type suspectedFixture struct {
	acceptFixture

	first statementImport
}

func newSuspectedFixture(t *testing.T, app *apitest.App, subject string) suspectedFixture {
	t.Helper()

	f := newImportFixture(t, app, subject)
	first := f.confirm(t, statement, "2500.00")

	pending := map[string]importedTransaction{}
	for _, i := range f.imported(t, "?state=PENDING").Items {
		pending[i.Description] = i
	}

	return suspectedFixture{acceptFixture: acceptFixture{importFixture: f, pending: pending}, first: first}
}

func (f importFixture) importedPath(id, action string) string {
	return "/households/" + f.household + "/imported-transactions/" + id + "/" + action
}

// suspected lists the Account's Suspected duplicates by description.
func (f importFixture) suspected(t *testing.T) map[string]importedTransaction {
	t.Helper()

	out := map[string]importedTransaction{}
	for _, i := range f.imported(t, "?state=SUSPECTED_DUPLICATE").Items {
		out[i.Description] = i
	}

	return out
}

func (f importFixture) decide(t *testing.T, id, action, wantState string) importedTransaction {
	t.Helper()

	var i importedTransaction
	f.app.Do(t, http.MethodPost, f.importedPath(id, action), f.token, nil).AssertJSON(t, http.StatusOK, &i)

	if i.State != wantState {
		t.Errorf("%s: state = %q, want %q", action, i.State, wantState)
	}

	return i
}

// importNamed returns the Connection's Statement import with the ID.
func (f importFixture) importNamed(t *testing.T, id string) statementImport {
	t.Helper()

	for _, si := range f.importList(t) {
		if si.StatementImportID == id {
			return si
		}
	}

	t.Fatalf("no Statement import %s", id)

	return statementImport{}
}

// Without bank transaction IDs nothing is a certain repeat: rows of a later
// file that only look like earlier ones (same Booking date, amount and
// description, ignoring case and spacing) are imported as Suspected
// duplicates, each listed next to the earlier row it resembles, and the
// import is recorded with its Balance check waiting (spec stories 39, 40,
// 40b, 43c).
func TestOverlappingFilesWithoutBankIDsBringSuspectedDuplicates(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	if si.RowsImported != 3 || si.RowsSkippedAsCertainRepeats != 0 || si.SuspectedDuplicates != 2 ||
		si.BalanceCheck != "WAITING" {
		t.Errorf("statement import = %+v, want 3 imported, none skipped, 2 Suspected duplicates, the check waiting", si)
	}

	suspected := f.suspected(t)
	if len(suspected) != 2 {
		t.Fatalf("suspected duplicates = %+v, want the salary and the coffee", suspected)
	}

	for description, earlier := range map[string]string{"SALARY": "Salary", "coffee": "Coffee"} {
		got := suspected[description]
		want := f.pending[earlier]

		if got.State != "SUSPECTED_DUPLICATE" || got.StatementImportID != si.StatementImportID || got.Resembles == nil {
			t.Fatalf("%s = %+v, want a Suspected duplicate of the later import, with what it resembles", description, got)
		}

		if r := got.Resembles; r.ImportedTransactionID != want.ImportedTransactionID || r.Description != earlier ||
			r.StatementImportID != f.first.StatementImportID || r.State != "PENDING" || r.BookingDate != want.BookingDate {
			t.Errorf("%s resembles %+v, want the first import's %s", description, r, earlier)
		}

		assertMoney(t, description+" resembles", got.Resembles.Amount, want.Amount.Amount.String())
	}

	if got := f.imported(t, "?state=PENDING"); len(got.Items) != 4 {
		t.Errorf("pending = %+v, want the first file's three and the groceries", got.Items)
	}

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "WAITING" || got.SuspectedDuplicates != 2 {
		t.Errorf("listed import = %+v, want the check waiting on 2 Suspected duplicates", got)
	}

	if got := f.importNamed(t, f.first.StatementImportID); got.SuspectedDuplicates != 0 || got.BalanceCheck != "PASSED" {
		t.Errorf("first import = %+v, want no Suspected duplicates and the check passed", got)
	}
}

// The same bank transaction ID with another Booking date or amount is not
// a certain repeat: the row is imported as a Suspected duplicate of the
// earlier one (spec story 39).
func TestTheSameBankIDWithAnotherDateOrAmountIsASuspectedDuplicate(t *testing.T) {
	t.Parallel()
	f := newLaterImportFixture(t, apitest.Start(t), "alice")

	// 510.50 + 1989.50 - 45.10 = 2454.90 once both are Discarded.
	var si statementImport
	f.upload(t, "", "Date;ID;Description;Amount\n"+
		"16/01/2026;A2;Salary;3.000,00\n"+
		"02/02/2026;A3;Coffee;-10,60\n"+
		"10/02/2026;A4;Groceries;-45,10\n", "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	if si.RowsImported != 3 || si.RowsSkippedAsCertainRepeats != 0 || si.SuspectedDuplicates != 2 ||
		si.BalanceCheck != "WAITING" {
		t.Errorf("statement import = %+v, want 3 imported, 2 Suspected duplicates, the check waiting", si)
	}

	suspected := f.suspected(t)

	for description, bankID := range map[string]string{"Salary": "A2", "Coffee": "A3"} {
		got := suspected[description]
		if got.Resembles == nil || got.Resembles.BankTransactionID != bankID || got.BankTransactionID != bankID {
			t.Errorf("%s = %+v, want a Suspected duplicate of the earlier %s", description, got, bankID)
		}
	}

	f.decide(t, suspected["Salary"].ImportedTransactionID, "discard", "DISCARDED")
	f.decide(t, suspected["Coffee"].ImportedTransactionID, "discard", "DISCARDED")

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "PASSED" {
		t.Errorf("check = %q, want passed once both are Discarded", got.BalanceCheck)
	}
}

// Identical rows within one file are never Suspected duplicates of each
// other: two equal coffees on one day both count, in a first import and in
// a later one (spec story 40e).
func TestIdenticalRowsWithinOneFileAreBothImported(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	twoCoffees := "Date;Description;Amount\n02/01/2026;Coffee;-2,00\n02/01/2026;Coffee;-2,00\n"

	if si := f.confirm(t, twoCoffees, "100.00"); si.RowsImported != 2 || si.SuspectedDuplicates != 0 {
		t.Errorf("first import = %+v, want both coffees imported", si)
	}

	// 104.00 - 4.00 + 2 × -1.50 = 97.00.
	var si statementImport
	f.upload(t, "", "Date;Description;Amount\n05/01/2026;Bus;-1,50\n05/01/2026;Bus;-1,50\n", "97.00", "").
		AssertJSON(t, http.StatusCreated, &si)

	if si.RowsImported != 2 || si.SuspectedDuplicates != 0 || si.BalanceCheck != "PASSED" {
		t.Errorf("later import = %+v, want both buses imported and the check passed", si)
	}

	if got := f.imported(t, "?state=PENDING"); len(got.Items) != 4 {
		t.Errorf("pending = %+v, want both coffees and both buses", got.Items)
	}
}

// Keeping a genuine second coffee makes it pending, to be Accepted like any
// other row; Discarding the repeated salary keeps it out. Once the last
// Suspected duplicate is decided the waiting check runs, and passes when
// the decisions meet the Export balance (spec stories 40c, 40d, 43c).
func TestAWaitingBalanceCheckPassesOnceEverySuspectedDuplicateIsDecided(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", overlapWithoutIDs, "2444.40", "").AssertJSON(t, http.StatusCreated, &si)

	suspected := f.suspected(t)

	f.decide(t, suspected["SALARY"].ImportedTransactionID, "discard", "DISCARDED")

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "WAITING" {
		t.Errorf("check = %q, want still waiting for the coffee", got.BalanceCheck)
	}

	kept := f.decide(t, suspected["coffee"].ImportedTransactionID, "keep", "PENDING")
	if kept.Resembles == nil || kept.Resembles.ImportedTransactionID != f.pending["Coffee"].ImportedTransactionID {
		t.Errorf("kept = %+v, want it still to show the coffee it resembled", kept)
	}

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "PASSED" {
		t.Errorf("check = %q, want passed", got.BalanceCheck)
	}

	if got := f.suspected(t); len(got) != 0 {
		t.Errorf("suspected duplicates = %+v, want none left", got)
	}

	if got := f.imported(t, "?state=PENDING"); len(got.Items) != 5 {
		t.Errorf("pending = %+v, want the first file's three, the kept coffee and the groceries", got.Items)
	}

	f.pending["kept coffee"] = kept
	f.accepted(t, "kept coffee", nil)

	if got := f.imported(t, "?state=ACCEPTED"); len(got.Items) != 1 || got.Items[0].ImportedTransactionID != kept.ImportedTransactionID {
		t.Errorf("accepted = %+v, want the kept coffee", got.Items)
	}
}

// When the decisions don't meet the Export balance, the waiting check runs
// and is recorded as failed: here the coffee is kept, but the bank's
// balance says it was a repeat (spec story 43c).
func TestAWaitingBalanceCheckFailsWhenTheDecisionsDontMeetTheExportBalance(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	suspected := f.suspected(t)
	f.decide(t, suspected["coffee"].ImportedTransactionID, "keep", "PENDING")

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "WAITING" {
		t.Errorf("check = %q, want still waiting for the salary", got.BalanceCheck)
	}

	f.decide(t, suspected["SALARY"].ImportedTransactionID, "discard", "DISCARDED")

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "FAILED" {
		t.Errorf("check = %q, want failed", got.BalanceCheck)
	}
}

// Two members deciding an import's last two Suspected duplicates at the
// same moment: exactly one of them finds none left, so the waiting check
// always runs (consistency map: Keep or Discard Imported transaction).
func TestDecidingTheLastSuspectedDuplicatesConcurrentlyRunsTheCheck(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	const rounds = 4

	var wg sync.WaitGroup
	for round := range rounds {
		wg.Go(func() {
			f := newSuspectedFixture(t, app, fmt.Sprintf("member-%d", round))

			var si statementImport
			f.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

			suspected := f.suspected(t)

			var decisions sync.WaitGroup
			for _, i := range suspected {
				decisions.Go(func() {
					f.app.Do(t, http.MethodPost, f.importedPath(i.ImportedTransactionID, "discard"), f.token, nil).
						AssertJSON(t, http.StatusOK, &importedTransaction{})
				})
			}

			decisions.Wait()

			if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "PASSED" {
				t.Errorf("round %d: check = %q, want passed", round, got.BalanceCheck)
			}
		})
	}

	wg.Wait()
}

// Only a Suspected duplicate is kept, a Suspected duplicate is decided
// before it is Accepted, and a decision is made once. A refusal changes
// nothing.
func TestKeepingAndDecidingAreRefused(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	f.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &statementImport{})

	suspected := f.suspected(t)
	salary, coffee := suspected["SALARY"].ImportedTransactionID, suspected["coffee"].ImportedTransactionID

	keep := func(id string) *apitest.Response {
		return f.app.Do(t, http.MethodPost, f.importedPath(id, "keep"), f.token, nil)
	}

	keep(f.pending["Rent"].ImportedTransactionID).
		AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-not-suspected-duplicate")
	f.app.Do(t, http.MethodPost, f.importedPath(salary, "accept"), f.token, nil).
		AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-suspected-duplicate")
	keep("0198f0a0-0000-7000-8000-000000000999").AssertProblem(t, http.StatusNotFound, "about:blank")

	if got := f.suspected(t); len(got) != 2 {
		t.Errorf("suspected duplicates = %+v, want both still undecided", got)
	}

	f.decide(t, salary, "keep", "PENDING")
	keep(salary).AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-not-suspected-duplicate")

	f.decide(t, coffee, "discard", "DISCARDED")
	keep(coffee).AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-not-suspected-duplicate")
	f.app.Do(t, http.MethodPost, f.importedPath(coffee, "discard"), f.token, nil).
		AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-already-discarded")
}

// Another Household can neither see nor decide Alice's Suspected
// duplicates.
func TestAnotherHouseholdCantDecideSuspectedDuplicates(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := newSuspectedFixture(t, app, "alice")
	bob := newSuspectedFixture(t, app, "bob")

	alice.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &statementImport{})

	salary := alice.suspected(t)["SALARY"].ImportedTransactionID

	for _, household := range []string{alice.household, bob.household} {
		for _, action := range []string{"keep", "discard"} {
			app.Do(t, http.MethodPost, "/households/"+household+"/imported-transactions/"+salary+"/"+action, bob.token, nil).
				AssertProblem(t, http.StatusNotFound, "about:blank")
		}
	}

	if got := alice.suspected(t); len(got) != 2 {
		t.Errorf("Alice's suspected duplicates = %+v, want both still undecided", got)
	}
}

// Retrying a Keep with its Idempotency-Key keeps once and answers the same.
func TestRetryingAKeepWithItsIdempotencyKeyKeepsOnce(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	f.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &statementImport{})
	path := f.importedPath(f.suspected(t)["coffee"].ImportedTransactionID, "keep")
	header := http.Header{"Idempotency-Key": []string{"keep-coffee"}}

	first := f.app.DoWithHeader(t, http.MethodPost, path, f.token, header, nil)
	again := f.app.DoWithHeader(t, http.MethodPost, path, f.token, header, nil)

	first.AssertJSON(t, http.StatusOK, &importedTransaction{})

	if again.Status != http.StatusOK || string(again.Body) != string(first.Body) {
		t.Errorf("replay = %d %s, want the first answer %s", again.Status, again.Body, first.Body)
	}
}

// What Suspected duplicates and their decisions recorded is rebuilt the
// same from the events.
func TestSuspectedDuplicatesSurviveARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newSuspectedFixture(t, app, "alice")

	f.upload(t, "", overlapWithoutIDs, "2444.40", "").AssertJSON(t, http.StatusCreated, &statementImport{})
	f.decide(t, f.suspected(t)["coffee"].ImportedTransactionID, "keep", "PENDING")

	before := []any{f.imported(t, ""), f.importList(t)}

	app.Rebuild(t)

	after := []any{f.imported(t, ""), f.importList(t)}
	if !sameJSON(t, after, before) {
		t.Errorf("after rebuild:\n  %+v\nbefore:\n  %+v", after, before)
	}

	f.decide(t, f.suspected(t)["SALARY"].ImportedTransactionID, "discard", "DISCARDED")

	for _, si := range f.importList(t) {
		if si.BalanceCheck != "PASSED" {
			t.Errorf("import %+v, want every check passed", si)
		}
	}
}
