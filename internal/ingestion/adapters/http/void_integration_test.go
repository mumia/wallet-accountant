//go:build integration

package http_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// voidedImport is a Statement import as voidStatementImport returns it.
type voidedImport struct {
	statementImport

	VoidedAt string `json:"voided_at"`
}

func (f importFixture) void(t *testing.T, statementImportID string) *apitest.Response {
	t.Helper()

	return f.app.Do(t, http.MethodPost, f.imports()+"/"+statementImportID+"/void", f.token, nil)
}

func (f importFixture) voided(t *testing.T, statementImportID string) voidedImport {
	t.Helper()

	var v voidedImport
	f.void(t, statementImportID).AssertJSON(t, http.StatusOK, &v)

	if v.StatementImportID != statementImportID || v.VoidedAt == "" {
		t.Fatalf("voided import = %+v, want %s with its voided_at", v, statementImportID)
	}

	return v
}

func (f importFixture) setMapping(t *testing.T, m mapping) {
	t.Helper()

	f.app.Put(t, f.connections()+"/"+f.connection+"/csv-mapping", f.token, m).AssertJSON(t, http.StatusOK, &connection{})
}

// statesOf returns the state of every Imported transaction of the Account
// by its description and statement import.
func (f importFixture) statesOf(t *testing.T, statementImportID string) map[string]string {
	t.Helper()

	states := map[string]string{}

	for _, i := range f.imported(t, "").Items {
		if i.StatementImportID == statementImportID {
			states[i.Description] = i.State
		}
	}

	return states
}

// withMemo is a later file with a memo column beside the description. Its
// Export balance is 510.50 + 1989.50 - 45.10 = 2454.90: A2 is a certain
// repeat.
const withMemo = "Date;ID;Description;Memo;Amount\n" +
	"15/01/2026;A2;Salary;Payroll;3.000,00\n" +
	"10/02/2026;A4;Groceries;Card 1234;-45,10\n"

// A later import read with a wrong mapping passes its Balance check when
// only the description is wrong. Voiding it Discards its rows and releases
// its file, so the same file is uploaded again once the mapping is fixed
// (spec stories 43d, 60).
func TestVoidAndReUploadWithAFixedMapping(t *testing.T) {
	t.Parallel()
	f := newLaterImportFixture(t, apitest.Start(t), "alice")

	wrong := withBankIDs()
	wrong["columns"].(map[string]any)["description"] = map[string]any{"header": "Memo"}
	f.setMapping(t, wrong)

	var bad statementImport
	f.upload(t, "", withMemo, "2454.90", "").AssertJSON(t, http.StatusCreated, &bad)

	if got := f.statesOf(t, bad.StatementImportID); got["Card 1234"] != "PENDING" {
		t.Fatalf("rows of the bad import = %v, want the memo as a pending description", got)
	}

	v := f.voided(t, bad.StatementImportID)
	if v.OpeningBalance != nil || v.RowsImported != 1 {
		t.Errorf("voided import = %+v, want the later import as it was", v)
	}

	if got := f.statesOf(t, bad.StatementImportID); len(got) != 1 || got["Card 1234"] != "DISCARDED" {
		t.Errorf("rows of the voided import = %v, want its row Discarded", got)
	}

	// The Account and its first import are untouched.
	if a := f.getAccount(t); a.AwaitingFirstStatementImport || a.Balance == nil || a.Balance.Amount.String() != "510.50" {
		t.Errorf("account = %+v, want its Opening balance kept", a)
	}

	f.setMapping(t, withBankIDs())

	// The same file again: its hash is released, and the Discarded row is
	// no certain repeat.
	var good statementImport
	f.upload(t, "", withMemo, "2454.90", "").AssertJSON(t, http.StatusCreated, &good)

	if good.RowsImported != 1 || good.RowsSkippedAsCertainRepeats != 1 || good.BalanceCheck != "PASSED" {
		t.Errorf("re-upload = %+v, want Groceries imported and the check passed", good)
	}

	if got := f.statesOf(t, good.StatementImportID); got["Groceries"] != "PENDING" {
		t.Errorf("rows of the re-upload = %v, want Groceries pending", got)
	}

	list := f.importList(t)
	if len(list) != 3 {
		t.Fatalf("statement imports = %+v, want the first, the voided and the re-upload", list)
	}

	var voidedList struct {
		Items []voidedImport `json:"items"`
	}

	f.app.Get(t, f.imports(), f.token).AssertJSON(t, http.StatusOK, &voidedList)

	for i, want := range []bool{false, true, false} {
		if got := voidedList.Items[i].VoidedAt != ""; got != want {
			t.Errorf("import %d voided = %v, want %v", i, got, want)
		}
	}

	// The re-upload is not a duplicate of the voided import, but uploading
	// it once more is.
	f.upload(t, "", withMemo, "2454.90", "").AssertProblem(t, http.StatusConflict, "/problems/duplicate-statement-import")
}

// Voiding the Account's only import also reverses its Opening balance: the
// Account awaits its first Statement import again, and a corrected first
// file is previewed and confirmed (spec stories 43e, 60, ADR-0018).
func TestVoidingTheFirstImportReopensTheOpeningBalance(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	inverted := signed()
	inverted["positive_amount_means"] = "MONEY_OUT"
	f.setMapping(t, inverted)

	// Inverted: 2500.00 + 1989.50 = 4489.50.
	first := f.confirm(t, statement, "2500.00")
	assertMoney(t, "wrong opening balance", *first.OpeningBalance, "4489.50")

	v := f.voided(t, first.StatementImportID)
	if v.OpeningBalance == nil {
		t.Errorf("voided import = %+v, want it to keep showing the Opening balance it had", v)
	}

	if a := f.getAccount(t); !a.AwaitingFirstStatementImport || a.Balance != nil {
		t.Errorf("account = %+v, want it awaiting its first Statement import again", a)
	}

	for d, s := range f.statesOf(t, first.StatementImportID) {
		if s != "DISCARDED" {
			t.Errorf("%s is %s, want Discarded", d, s)
		}
	}

	// The Opening balance Transaction and its reversal are both listed,
	// linked to each other.
	af := acceptFixture{importFixture: f}

	lines := af.listed(t, "")
	if len(lines) != 2 || lines[0].ReversedByTransactionID != lines[1].TransactionID ||
		lines[1].ReversalOfTransactionID != lines[0].TransactionID || lines[1].Amount.Amount.String() != "-4489.50" {
		t.Fatalf("transactions = %+v, want the Opening balance and its reversal", lines)
	}

	f.setMapping(t, signed())

	p := f.previewed(t, statement, "2500.00")
	assertMoney(t, "previewed opening balance", p.OpeningBalance, "510.50")

	again := f.confirm(t, statement, "2500.00")
	assertMoney(t, "opening balance", *again.OpeningBalance, "510.50")

	if a := f.getAccount(t); a.AwaitingFirstStatementImport || a.Balance == nil || a.Balance.Amount.String() != "510.50" {
		t.Errorf("account = %+v, want the corrected Opening balance", a)
	}

	if got := f.statesOf(t, again.StatementImportID); len(got) != 3 || got["Salary"] != "PENDING" {
		t.Errorf("rows of the corrected import = %v, want three pending", got)
	}

	// The corrected Opening balance can be voided again, and the reversal
	// of the first one is untouched.
	if got := af.listed(t, ""); len(got) != 3 {
		t.Errorf("transactions = %+v, want the old Opening balance, its reversal and the new one", got)
	}

	// A later import's Balance check counts only the new Opening balance
	// and the corrected rows: 510.50 + 1989.50 - 45.10 = 2454.90.
	f.upload(t, "", otherStatement, "2454.90", "").AssertJSON(t, http.StatusCreated, &statementImport{})
}

// januaryBonus is a later file with one new January row: 510.50 + 1989.50
// + 100.00 = 2600.00.
const januaryBonus = "Date;Description;Amount\n20/01/2026;Bonus;100,00\n"

// Voiding the first import while a later file is uploaded: either the
// later import is recorded first, and the first is no longer the latest so
// it can't be voided; or the void runs first, the Account awaits its first
// Statement import again, and the later upload is refused as needing a
// preview. Never both, which would record a later import on an Account
// with no Opening balance.
func TestVoidingTheFirstImportWhileALaterOneIsUploaded(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	const rounds = 6

	var wg sync.WaitGroup
	for round := range rounds {
		wg.Go(func() {
			f := newImportFixture(t, app, fmt.Sprintf("member-%d", round))
			first := f.confirm(t, statement, "2500.00")

			var void, upload *apitest.Response

			var both sync.WaitGroup
			both.Go(func() { void = f.void(t, first.StatementImportID) })
			both.Go(func() { upload = f.upload(t, "", januaryBonus, "2600.00", "") })
			both.Wait()

			if upload.Status == http.StatusCreated {
				void.AssertProblem(t, http.StatusConflict, "/problems/not-the-latest-statement-import")

				return
			}

			upload.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/preview-required")
			void.AssertJSON(t, http.StatusOK, &voidedImport{})

			if a := f.getAccount(t); !a.AwaitingFirstStatementImport {
				t.Errorf("round %d: account = %+v, want it awaiting its first Statement import", round, a)
			}
		})
	}

	wg.Wait()
}

// Only the Account's latest import can be voided, so every remaining
// import's check stays valid; voiding the later one first makes the
// earlier one the latest (spec story 43f).
func TestVoidingAnEarlierImportIsRefused(t *testing.T) {
	t.Parallel()
	f := newLaterImportFixture(t, apitest.Start(t), "alice")

	first := f.importList(t)[0]

	var later statementImport
	f.upload(t, "", overlapping, "2454.90", "").AssertJSON(t, http.StatusCreated, &later)

	f.void(t, first.StatementImportID).AssertProblem(t, http.StatusConflict, "/problems/not-the-latest-statement-import")

	if a := f.getAccount(t); a.AwaitingFirstStatementImport {
		t.Errorf("account = %+v, want its Opening balance kept after the refusal", a)
	}

	if got := f.statesOf(t, first.StatementImportID); got["Rent"] != "PENDING" {
		t.Errorf("rows of the first import = %v, want them untouched", got)
	}

	f.voided(t, later.StatementImportID)
	f.void(t, later.StatementImportID).AssertProblem(t, http.StatusConflict, "/problems/statement-import-already-voided")
	f.voided(t, first.StatementImportID)

	if a := f.getAccount(t); !a.AwaitingFirstStatementImport {
		t.Errorf("account = %+v, want it awaiting its first Statement import", a)
	}
}

// An import with an Accepted row can't be voided: the Accept is reversed
// first, and then the void Discards the row it returned to pending.
func TestVoidingAnImportWithAcceptedRowsIsRefused(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	first := f.importList(t)[0]
	salary := f.accepted(t, "Salary", nil)

	f.void(t, first.StatementImportID).
		AssertProblem(t, http.StatusConflict, "/problems/statement-import-has-accepted-rows")

	if got := f.statesOf(t, first.StatementImportID); got["Salary"] != "ACCEPTED" || got["Rent"] != "PENDING" {
		t.Errorf("rows = %v, want them untouched", got)
	}

	f.assertBalance(t, "3510.50")

	f.reversed(t, salary.TransactionID)
	f.voided(t, first.StatementImportID)

	if got := f.statesOf(t, first.StatementImportID); got["Salary"] != "DISCARDED" || got["Rent"] != "DISCARDED" {
		t.Errorf("rows = %v, want every row Discarded", got)
	}

	if a := f.getAccount(t); !a.AwaitingFirstStatementImport || a.Balance != nil {
		t.Errorf("account = %+v, want it awaiting its first Statement import", a)
	}
}

// Voiding is replayed for the same Idempotency-Key, and another Household
// or a wrong Connection gets 404.
func TestVoidIsIdempotentAndScopedToTheHousehold(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := newImportFixture(t, app, "alice")
	bob := newImportFixture(t, app, "bob")
	first := alice.confirm(t, statement, "2500.00")
	bob.confirm(t, statement, "2500.00")

	for _, path := range []string{
		"/api/households/" + alice.household + "/connections/" + alice.connection + "/statement-imports/" + first.StatementImportID + "/void",
		"/api/households/" + bob.household + "/connections/" + bob.connection + "/statement-imports/" + first.StatementImportID + "/void",
		"/api/households/" + bob.household + "/connections/" + alice.connection + "/statement-imports/" + first.StatementImportID + "/void",
	} {
		app.Do(t, http.MethodPost, path, bob.token, nil).AssertProblem(t, http.StatusNotFound, "about:blank")
	}

	// Alice through another of her Connections.
	other := alice.create(t, map[string]any{"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed()}).ConnectionID
	app.Do(t, http.MethodPost, "/api/households/"+alice.household+"/connections/"+other+"/statement-imports/"+
		first.StatementImportID+"/void", alice.token, nil).AssertProblem(t, http.StatusNotFound, "about:blank")
	alice.void(t, "0198f0a0-0000-7000-8000-00000000dead").AssertProblem(t, http.StatusNotFound, "about:blank")

	path := alice.imports() + "/" + first.StatementImportID + "/void"

	var once, twice voidedImport
	app.DoWithHeader(t, http.MethodPost, path, alice.token, http.Header{"Idempotency-Key": {"void-1"}}, nil).AssertJSON(t, http.StatusOK, &once)
	app.DoWithHeader(t, http.MethodPost, path, alice.token, http.Header{"Idempotency-Key": {"void-1"}}, nil).AssertJSON(t, http.StatusOK, &twice)

	if once.VoidedAt == "" || once.VoidedAt != twice.VoidedAt {
		t.Errorf("replay = %+v, want %+v", twice, once)
	}

	if a := bob.getAccount(t); a.AwaitingFirstStatementImport {
		t.Errorf("Bob's account = %+v, want it untouched", a)
	}
}

// What a void recorded is rebuilt the same from the events.
func TestVoidSurvivesARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newImportFixture(t, app, "alice")
	f.voided(t, f.confirm(t, statement, "2500.00").StatementImportID)

	af := acceptFixture{importFixture: f}
	before := []any{f.getAccount(t), af.listed(t, ""), f.imported(t, ""), f.importList(t)}

	app.Rebuild(t)

	after := []any{f.getAccount(t), af.listed(t, ""), f.imported(t, ""), f.importList(t)}
	if !sameJSON(t, after, before) {
		t.Errorf("after rebuild:\n  %+v\nbefore:\n  %+v", after, before)
	}

	// The file is still released after the rebuild.
	f.confirm(t, statement, "2500.00")
}

// Voiding a later import whose Balance check still waits on undecided
// Suspected duplicates Discards every row, kept or still suspected, and its
// check never runs: deciding a row afterwards is refused. Its file is
// released, so uploading it again brings the Suspected duplicates back
// (spec stories 40, 43c, 43d).
func TestVoidingAnImportWithUndecidedSuspectedDuplicates(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", overlapWithoutIDs, "2444.40", "").AssertJSON(t, http.StatusCreated, &si)

	suspected := f.suspected(t)
	f.decide(t, suspected["coffee"].ImportedTransactionID, "keep", "PENDING")

	v := f.voided(t, si.StatementImportID)
	if v.BalanceCheck != "WAITING" || v.SuspectedDuplicates != 2 {
		t.Errorf("voided import = %+v, want it as it was, its check waiting", v)
	}

	got := f.statesOf(t, si.StatementImportID)
	if len(got) != 3 {
		t.Fatalf("rows of the voided import = %v, want its three rows", got)
	}

	for description, state := range got {
		if state != "DISCARDED" {
			t.Errorf("%s = %s, want Discarded", description, state)
		}
	}

	f.app.Do(t, http.MethodPost, f.importedPath(suspected["SALARY"].ImportedTransactionID, "discard"), f.token, nil).
		AssertProblem(t, http.StatusConflict, "/problems/imported-transaction-already-discarded")

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "WAITING" {
		t.Errorf("voided import's check = %q, want it never run", got.BalanceCheck)
	}

	var again statementImport
	f.upload(t, "", overlapWithoutIDs, "2444.40", "").AssertJSON(t, http.StatusCreated, &again)

	if again.RowsImported != 3 || again.SuspectedDuplicates != 2 || again.BalanceCheck != "WAITING" {
		t.Errorf("re-upload = %+v, want 3 imported, 2 Suspected duplicates, the check waiting", again)
	}

	if got := f.suspected(t); len(got) != 2 || got["SALARY"].StatementImportID != again.StatementImportID {
		t.Errorf("suspected duplicates = %+v, want the re-upload's salary and coffee", got)
	}
}
