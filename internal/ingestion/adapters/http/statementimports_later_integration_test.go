//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// withBankIDs maps a file whose second column is the bank's own
// transaction ID.
func withBankIDs() mapping {
	m := signed()
	m["columns"].(map[string]any)["bank_transaction_id"] = map[string]any{"header": "ID"}

	return m
}

// firstWithIDs is an Account's first file: Opening balance 2500.00 -
// 1989.50 = 510.50.
const firstWithIDs = "Date;ID;Description;Amount\n" +
	"01/01/2026;A1;Rent;-1.000,00\n" +
	"15/01/2026;A2;Salary;3.000,00\n" +
	"02/02/2026;A3;Coffee;-10,50\n"

// overlapping is the next file: it repeats A2 and A3, brings one new row,
// and has one row before the start date. Its Export balance is
// 510.50 + 1989.50 - 45.10 = 2454.90.
const overlapping = "Date;ID;Description;Amount\n" +
	"31/12/2025;A0;Before the start date;-5,00\n" +
	"15/01/2026;A2;Salary;3.000,00\n" +
	"02/02/2026;A3;Coffee;-10,50\n" +
	"10/02/2026;A4;Groceries;-45,10\n"

type balanceCheckProblem struct {
	ExpectedBalance *money `json:"expected_balance"`
	ExportBalance   *money `json:"export_balance"`
	Difference      *money `json:"difference"`
}

// newLaterImportFixture is an import fixture whose Connection reads bank
// transaction IDs and whose Account already has its first import.
func newLaterImportFixture(t *testing.T, app *apitest.App, subject string) importFixture {
	t.Helper()

	f := newImportFixture(t, app, subject)
	f.connection = f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT", "csv_mapping": withBankIDs()}).ConnectionID
	f.confirm(t, firstWithIDs, "2500.00")

	return f
}

// A later import of an overlapping file skips the rows already imported
// (same bank transaction ID, Booking date and amount), imports the rest as
// pending, and passes its Balance check: the Opening balance plus every
// Imported transaction is its Export balance (spec stories 38, 40, 43a).
func TestALaterImportSkipsCertainRepeatsAndPassesItsBalanceCheck(t *testing.T) {
	t.Parallel()
	f := newLaterImportFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", overlapping, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	if si.RowsImported != 1 || si.RowsSkippedAsCertainRepeats != 2 || si.RowsIgnoredBeforeStartDate != 1 ||
		si.BalanceCheck != "PASSED" || si.OpeningBalance != nil {
		t.Errorf("statement import = %+v, want 1 imported, 2 skipped, 1 ignored, the check passed, no Opening balance", si)
	}

	assertMoney(t, "export balance", si.ExportBalance, "2454.90")
	assertMoney(t, "rows total", si.RowsTotal, "-45.10")

	pending := f.imported(t, "?state=PENDING")
	if len(pending.Items) != 4 {
		t.Fatalf("pending = %+v, want the first file's three rows and the new one", pending.Items)
	}

	if got := pending.Items[3]; got.Description != "Groceries" || got.StatementImportID != si.StatementImportID {
		t.Errorf("new row = %+v, want the groceries from the later import", got)
	}

	if got := f.importList(t); len(got) != 2 || got[1].StatementImportID != si.StatementImportID ||
		got[0].RowsSkippedAsCertainRepeats != 0 {
		t.Errorf("statement imports = %+v, want the first and the later one", got)
	}

	// The Account's Opening balance is untouched.
	if a := f.getAccount(t); a.Balance == nil || a.Balance.Amount.String() != "510.50" {
		t.Errorf("account = %+v, want its balance still the Opening balance", a)
	}

	// A third file that only repeats rows adds nothing and still checks.
	var again statementImport
	f.upload(t, "", "Date;ID;Description;Amount\n10/02/2026;A4;Groceries;-45,10\n", "2454.90", "").
		AssertJSON(t, http.StatusCreated, &again)

	if again.RowsImported != 0 || again.RowsSkippedAsCertainRepeats != 1 || again.BalanceCheck != "PASSED" {
		t.Errorf("third import = %+v, want only a skipped repeat", again)
	}
}

// A later import whose rows don't add up to its Export balance is refused
// whole, showing the expected balance, the Export balance and the
// difference, and records nothing (spec story 43b).
func TestALaterImportFailingItsBalanceCheckIsRefused(t *testing.T) {
	t.Parallel()
	f := newLaterImportFixture(t, apitest.Start(t), "alice")

	res := f.upload(t, "", overlapping, "2400.00", "")
	res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/balance-check-failed")

	var p balanceCheckProblem
	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatal(err)
	}

	if p.ExpectedBalance == nil || p.ExportBalance == nil || p.Difference == nil {
		t.Fatalf("problem = %s, want the expected balance, the Export balance and the difference", res.Body)
	}

	assertMoney(t, "expected balance", *p.ExpectedBalance, "2454.90")
	assertMoney(t, "export balance", *p.ExportBalance, "2400.00")
	assertMoney(t, "difference", *p.Difference, "-54.90")

	f.assertOnlyTheFirstImport(t)

	// The right Export balance then passes: the refusal kept no file hash.
	f.upload(t, "", overlapping, "2454.90", "").AssertJSON(t, http.StatusCreated, &statementImport{})
}

// A wrong mapping is caught by the next import's check, because the
// mapping is reused (ADR-0018): with the sign inverted, the new rows add up
// to the wrong balance. When the file overlaps, the inverted rows only look
// like the earlier ones (same bank ID, another amount): they are Suspected
// duplicates, and the check, once they are Discarded as repeats, fails.
func TestAWrongMappingIsCaughtByTheBalanceCheck(t *testing.T) {
	t.Parallel()
	f := newLaterImportFixture(t, apitest.Start(t), "alice")

	inverted := withBankIDs()
	inverted["positive_amount_means"] = "MONEY_OUT"
	f.app.Put(t, f.connections()+"/"+f.connection+"/csv-mapping", f.token, inverted).AssertJSON(t, http.StatusOK, &connection{})

	res := f.upload(t, "", "Date;ID;Description;Amount\n10/02/2026;A4;Groceries;-45,10\n", "2454.90", "")
	res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/balance-check-failed")

	var p balanceCheckProblem
	if err := json.Unmarshal(res.Body, &p); err != nil || p.ExpectedBalance == nil || p.Difference == nil {
		t.Fatalf("problem = %s (%v)", res.Body, err)
	}

	// 510.50 + 1989.50 + 45.10 = 2545.10.
	assertMoney(t, "expected balance", *p.ExpectedBalance, "2545.10")
	assertMoney(t, "difference", *p.Difference, "-90.20")

	f.assertOnlyTheFirstImport(t)

	var si statementImport
	f.upload(t, "", overlapping, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	if si.SuspectedDuplicates != 2 || si.BalanceCheck != "WAITING" {
		t.Fatalf("overlapping import = %+v, want the inverted salary and coffee as Suspected duplicates", si)
	}

	for _, i := range f.suspected(t) {
		f.decide(t, i.ImportedTransactionID, "discard", "DISCARDED")
	}

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "FAILED" {
		t.Errorf("check = %q, want failed: the groceries came in with the wrong sign", got.BalanceCheck)
	}
}

// Later imports ask for the Export balance once and take no preview: a
// preview_id is refused, since only a first import has one.
func TestALaterImportRefusesAPreviewID(t *testing.T) {
	t.Parallel()
	f := newLaterImportFixture(t, apitest.Start(t), "alice")

	f.upload(t, "", overlapping, "2454.90", "a-preview").
		AssertProblem(t, http.StatusConflict, "/problems/opening-balance-already-recorded")
	f.preview(t, overlapping, "2454.90", "2454.90").
		AssertProblem(t, http.StatusConflict, "/problems/opening-balance-already-recorded")

	f.assertOnlyTheFirstImport(t)
}

func (f importFixture) assertOnlyTheFirstImport(t *testing.T) {
	t.Helper()

	if got := f.importList(t); len(got) != 1 {
		t.Errorf("statement imports = %+v, want only the first", got)
	}

	if got := f.imported(t, ""); len(got.Items) != 3 {
		t.Errorf("imported transactions = %+v, want only the first file's three", got.Items)
	}
}

// Accepted rows still count: once a row of the first import is Accepted,
// the next import's Balance check still adds it in, and it is still a
// certain repeat. A wrong Export balance is refused, the right one passes.
func TestALaterImportsBalanceCheckCountsAcceptedRows(t *testing.T) {
	t.Parallel()
	f := newLaterImportFixture(t, apitest.Start(t), "alice")

	pending := map[string]importedTransaction{}
	for _, i := range f.imported(t, "?state=PENDING").Items {
		pending[i.Description] = i
	}

	af := acceptFixture{importFixture: f, pending: pending}
	af.accepted(t, "Salary", map[string]any{})
	af.accepted(t, "Rent", map[string]any{})

	res := f.upload(t, "", overlapping, "2400.00", "")
	res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/balance-check-failed")

	var p balanceCheckProblem
	if err := json.Unmarshal(res.Body, &p); err != nil || p.ExpectedBalance == nil || p.Difference == nil {
		t.Fatalf("problem = %s (%v)", res.Body, err)
	}

	assertMoney(t, "expected balance", *p.ExpectedBalance, "2454.90")
	assertMoney(t, "difference", *p.Difference, "-54.90")
	f.assertOnlyTheFirstImport(t)

	var si statementImport
	f.upload(t, "", overlapping, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	if si.RowsImported != 1 || si.RowsSkippedAsCertainRepeats != 2 || si.BalanceCheck != "PASSED" {
		t.Errorf("statement import = %+v, want 1 imported, 2 skipped (one Accepted), the check passed", si)
	}

	if got := f.imported(t, "?state=ACCEPTED"); len(got.Items) != 2 {
		t.Errorf("accepted = %+v, want Rent and Salary still Accepted", got.Items)
	}
}
