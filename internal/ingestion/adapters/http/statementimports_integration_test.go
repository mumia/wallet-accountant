//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// The Account the fixture adds starts on 2026-01-01, in EUR.

// statement is a file for the signed() mapping: one row before the start
// date and three on or after it, totalling 1989.50.
const statement = "Date;Description;Amount\n" +
	"31/12/2025;Before the start date;-99,99\n" +
	"01/01/2026;Rent;-1.000,00\n" +
	"15/01/2026;Salary;3.000,00\n" +
	"02/02/2026;Coffee;-10,50\n"

const otherStatement = "Date;Description;Amount\n" +
	"05/01/2026;Groceries;-45,10\n"

type money struct {
	Amount   json.Number `json:"amount"`
	Currency string      `json:"currency"`
}

type preview struct {
	PreviewID                  string `json:"preview_id"`
	OpeningBalance             money  `json:"opening_balance"`
	StartDate                  string `json:"start_date"`
	ExportBalance              money  `json:"export_balance"`
	RowCount                   int    `json:"row_count"`
	RowsIgnoredBeforeStartDate int    `json:"rows_ignored_before_start_date"`
	RowsTotal                  money  `json:"rows_total"`
}

type statementImport struct {
	StatementImportID           string `json:"statement_import_id"`
	ConnectionID                string `json:"connection_id"`
	AccountID                   string `json:"account_id"`
	ExportBalance               money  `json:"export_balance"`
	RowsImported                int    `json:"rows_imported"`
	RowsSkippedAsCertainRepeats int    `json:"rows_skipped_as_certain_repeats"`
	SuspectedDuplicates         int    `json:"suspected_duplicates"`
	RowsIgnoredBeforeStartDate  int    `json:"rows_ignored_before_start_date"`
	RowsTotal                   money  `json:"rows_total"`
	OpeningBalance              *money `json:"opening_balance"`
	BalanceCheck                string `json:"balance_check"`
	ImportedAt                  string `json:"imported_at"`
}

type account struct {
	AwaitingFirstStatementImport bool   `json:"awaiting_first_statement_import"`
	Balance                      *money `json:"balance"`
}

type accountTransaction struct {
	TransactionID   string `json:"transaction_id"`
	Amount          money  `json:"amount"`
	BookingDate     string `json:"booking_date"`
	TransactionDate string `json:"transaction_date"`
	Description     string `json:"description"`
	CategoryID      string `json:"category_id"`
	CategoryName    string `json:"category_name"`
}

type importedTransaction struct {
	ImportedTransactionID string     `json:"imported_transaction_id"`
	StatementImportID     string     `json:"statement_import_id"`
	State                 string     `json:"state"`
	Line                  int        `json:"line"`
	BookingDate           string     `json:"booking_date"`
	TransactionDate       string     `json:"transaction_date"`
	Amount                money      `json:"amount"`
	Description           string     `json:"description"`
	BankTransactionID     string     `json:"bank_transaction_id"`
	Resembles             *resembled `json:"resembles"`
}

// resembled is the earlier Imported transaction a Suspected duplicate
// looks like.
type resembled struct {
	ImportedTransactionID string `json:"imported_transaction_id"`
	StatementImportID     string `json:"statement_import_id"`
	State                 string `json:"state"`
	BookingDate           string `json:"booking_date"`
	Amount                money  `json:"amount"`
	Description           string `json:"description"`
	BankTransactionID     string `json:"bank_transaction_id"`
}

type page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor"`
}

type rowsProblem struct {
	Rows []struct {
		Line   int    `json:"line"`
		Reason string `json:"reason"`
	} `json:"rows"`
}

// importFixture is a Household with an Account and a statement file import
// Connection whose mapping reads statement.
type importFixture struct {
	fixture

	connection string
}

func newImportFixture(t *testing.T, app *apitest.App, subject string) importFixture {
	t.Helper()

	f := newFixture(t, app, subject)
	c := f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed()})

	return importFixture{fixture: f, connection: c.ConnectionID}
}

func (f importFixture) imports() string {
	return "/api/households/" + f.household + "/connections/" + f.connection + "/statement-imports"
}

func (f importFixture) accountPath() string {
	return "/api/households/" + f.household + "/accounts/" + f.account
}

func (f importFixture) preview(t *testing.T, file, balance, confirmation string) *apitest.Response {
	t.Helper()

	return f.app.PostForm(t, f.imports()+"/previews", f.token, "", apitest.Form{
		Fields:   map[string]string{"export_balance": balance, "export_balance_confirmation": confirmation},
		FileName: "statement.csv",
		File:     []byte(file),
	})
}

func (f importFixture) previewed(t *testing.T, file, balance string) preview {
	t.Helper()

	var p preview
	f.preview(t, file, balance, balance).AssertJSON(t, http.StatusOK, &p)

	return p
}

func (f importFixture) upload(t *testing.T, key, file, balance, previewID string) *apitest.Response {
	t.Helper()

	fields := map[string]string{"export_balance": balance}
	if previewID != "" {
		fields["preview_id"] = previewID
	}

	return f.app.PostForm(t, f.imports(), f.token, key, apitest.Form{Fields: fields, FileName: "statement.csv", File: []byte(file)})
}

// confirm previews the file and confirms it.
func (f importFixture) confirm(t *testing.T, file, balance string) statementImport {
	t.Helper()

	var si statementImport
	f.upload(t, "", file, balance, f.previewed(t, file, balance).PreviewID).AssertJSON(t, http.StatusCreated, &si)

	return si
}

func (f importFixture) getAccount(t *testing.T) account {
	t.Helper()

	var a account
	f.app.Get(t, f.accountPath(), f.token).AssertJSON(t, http.StatusOK, &a)

	return a
}

func (f importFixture) transactions(t *testing.T, query string) page[accountTransaction] {
	t.Helper()

	var p page[accountTransaction]
	f.app.Get(t, f.accountPath()+"/transactions"+query, f.token).AssertJSON(t, http.StatusOK, &p)

	return p
}

func (f importFixture) imported(t *testing.T, query string) page[importedTransaction] {
	t.Helper()

	var p page[importedTransaction]
	f.app.Get(t, f.accountPath()+"/imported-transactions"+query, f.token).AssertJSON(t, http.StatusOK, &p)

	return p
}

func (f importFixture) importList(t *testing.T) []statementImport {
	t.Helper()

	var l page[statementImport]
	f.app.Get(t, f.imports(), f.token).AssertJSON(t, http.StatusOK, &l)

	return l.Items
}

// assertNothingImported checks that the Account still awaits its first
// Statement import and that nothing was recorded.
func (f importFixture) assertNothingImported(t *testing.T) {
	t.Helper()

	if a := f.getAccount(t); !a.AwaitingFirstStatementImport || a.Balance != nil {
		t.Errorf("account = %+v, want it still awaiting its first Statement import with no balance", a)
	}

	if got := f.importList(t); len(got) != 0 {
		t.Errorf("statement imports = %+v, want none", got)
	}

	if got := f.imported(t, ""); len(got.Items) != 0 {
		t.Errorf("imported transactions = %+v, want none", got.Items)
	}

	if got := f.transactions(t, ""); len(got.Items) != 0 {
		t.Errorf("transactions = %+v, want none", got.Items)
	}
}

func assertMoney(t *testing.T, name string, got money, want string) {
	t.Helper()

	if got.Amount.String() != want || got.Currency != "EUR" {
		t.Errorf("%s = %s %s, want %s EUR", name, got.Amount, got.Currency, want)
	}
}

// A preview shows the Opening balance the first import would record: the
// Export balance minus the rows on or after the start date. Amounts carry
// the currency's two decimal places, trailing zeros included.
func TestPreviewingAFirstImportRecordsNothing(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	res := f.preview(t, statement, "2500", "2500.00")

	var p preview
	res.AssertJSON(t, http.StatusOK, &p)

	if p.PreviewID == "" || p.StartDate != "2026-01-01" || p.RowCount != 3 || p.RowsIgnoredBeforeStartDate != 1 {
		t.Errorf("preview = %+v", p)
	}

	assertMoney(t, "export balance", p.ExportBalance, "2500.00")
	assertMoney(t, "rows total", p.RowsTotal, "1989.50")
	assertMoney(t, "opening balance", p.OpeningBalance, "510.50")

	if !strings.Contains(string(res.Body), `"export_balance":{"amount":2500.00,"currency":"EUR"}`) {
		t.Errorf("the Export balance isn't written with two decimal places: %s", res.Body)
	}

	f.assertNothingImported(t)
}

func TestPreviewRefusesExportBalanceEntriesThatDiffer(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	f.preview(t, statement, "2500.00", "2050.00").AssertProblem(t, http.StatusUnprocessableEntity, "/problems/export-balance-mismatch")
	f.preview(t, statement, "2500.001", "2500.001").AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-export-balance")
	f.preview(t, statement, "2.500,00", "2.500,00").AssertProblem(t, http.StatusBadRequest, "about:blank")

	f.assertNothingImported(t)
}

// Confirming records the import, a pending Imported transaction per row on
// or after the start date, and the Opening balance Transaction on the start
// date, so the Account shows its balance and its single-entry list shows the
// Opening balance (ADR-0018).
func TestConfirmingAFirstImportRecordsItsOpeningBalance(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	p := f.previewed(t, statement, "2500.00")

	res := f.upload(t, "", statement, "2500.00", p.PreviewID)

	var si statementImport
	res.AssertJSON(t, http.StatusCreated, &si)

	if si.StatementImportID == "" || si.ConnectionID != f.connection || si.AccountID != f.account ||
		si.RowsImported != 3 || si.RowsIgnoredBeforeStartDate != 1 || si.BalanceCheck != "PASSED" || si.ImportedAt == "" {
		t.Errorf("statement import = %+v", si)
	}

	assertMoney(t, "rows total", si.RowsTotal, "1989.50")

	if si.OpeningBalance == nil {
		t.Fatal("the first import has no Opening balance")
	}

	assertMoney(t, "opening balance", *si.OpeningBalance, "510.50")

	a := f.getAccount(t)
	if a.AwaitingFirstStatementImport || a.Balance == nil {
		t.Fatalf("account = %+v, want a balance and no longer awaiting its first import", a)
	}

	assertMoney(t, "balance", *a.Balance, "510.50")

	tx := f.transactions(t, "?booking_month=2026-01")
	if len(tx.Items) != 1 || tx.NextCursor != "" {
		t.Fatalf("January transactions = %+v, want only the Opening balance", tx)
	}

	ob := tx.Items[0]
	if ob.BookingDate != "2026-01-01" || ob.TransactionDate != "2026-01-01" || ob.Description != "Opening balance" ||
		ob.CategoryName != "Opening balance" || ob.CategoryID == "" {
		t.Errorf("opening balance transaction = %+v", ob)
	}

	assertMoney(t, "opening balance transaction", ob.Amount, "510.50")

	pending := f.imported(t, "?state=PENDING")
	if len(pending.Items) != 3 {
		t.Fatalf("pending = %+v, want the three rows on or after the start date", pending.Items)
	}

	want := []struct {
		line                      int
		date, description, amount string
	}{
		{3, "2026-01-01", "Rent", "-1000.00"},
		{4, "2026-01-15", "Salary", "3000.00"},
		{5, "2026-02-02", "Coffee", "-10.50"},
	}
	for i, w := range want {
		got := pending.Items[i]
		if got.Line != w.line || got.BookingDate != w.date || got.TransactionDate != w.date ||
			got.Description != w.description || got.State != "PENDING" || got.StatementImportID != si.StatementImportID {
			t.Errorf("pending[%d] = %+v, want %+v", i, got, w)
		}

		assertMoney(t, "pending amount", got.Amount, w.amount)
	}

	if got := f.imported(t, "?state=PENDING&booking_month=2026-02"); len(got.Items) != 1 || got.Items[0].Description != "Coffee" {
		t.Errorf("February pending = %+v, want only the coffee", got.Items)
	}

	if got := f.importList(t); len(got) != 1 || got[0].StatementImportID != si.StatementImportID {
		t.Errorf("statement imports = %+v, want the one recorded", got)
	}
}

func TestConfirmingIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	t.Run("without preview_id", func(t *testing.T) {
		t.Parallel()
		f := newImportFixture(t, app, "no-preview")

		f.upload(t, "", statement, "2500.00", "").AssertProblem(t, http.StatusUnprocessableEntity, "/problems/preview-required")
		f.assertNothingImported(t)
	})

	t.Run("a different file", func(t *testing.T) {
		t.Parallel()
		f := newImportFixture(t, app, "other-file")

		p := f.previewed(t, statement, "2500.00")
		f.upload(t, "", otherStatement, "2500.00", p.PreviewID).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/preview-mismatch")
		f.assertNothingImported(t)
	})

	t.Run("a different Export balance", func(t *testing.T) {
		t.Parallel()
		f := newImportFixture(t, app, "other-balance")

		p := f.previewed(t, statement, "2500.00")
		f.upload(t, "", statement, "2500.01", p.PreviewID).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/preview-mismatch")
		f.assertNothingImported(t)
	})

	t.Run("the mapping changed since the preview", func(t *testing.T) {
		t.Parallel()
		f := newImportFixture(t, app, "changed-mapping")

		p := f.previewed(t, statement, "2500.00")

		f.app.Put(t, f.connections()+"/"+f.connection+"/csv-mapping", f.token, signed()).AssertJSON(t, http.StatusOK, &connection{})
		f.upload(t, "", statement, "2500.00", p.PreviewID).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/preview-mismatch")
		f.assertNothingImported(t)
	})

	t.Run("the Account got its Opening balance since the preview", func(t *testing.T) {
		t.Parallel()
		f := newImportFixture(t, app, "stale-preview")

		stale := f.previewed(t, otherStatement, "100.00")
		f.confirm(t, statement, "2500.00")

		f.upload(t, "", otherStatement, "100.00", stale.PreviewID).
			AssertProblem(t, http.StatusConflict, "/problems/opening-balance-already-recorded")
		f.preview(t, otherStatement, "100.00", "100.00").
			AssertProblem(t, http.StatusConflict, "/problems/opening-balance-already-recorded")

		if got := f.importList(t); len(got) != 1 {
			t.Errorf("statement imports = %+v, want only the first", got)
		}

		if a := f.getAccount(t); a.Balance == nil || a.Balance.Amount.String() != "510.50" {
			t.Errorf("account = %+v, want the first import's Opening balance kept", a)
		}
	})

	t.Run("a Connection without a mapping", func(t *testing.T) {
		t.Parallel()
		f := newImportFixture(t, app, "no-mapping")
		f.connection = f.create(t, map[string]any{"kind": "STATEMENT_FILE_IMPORT"}).ConnectionID

		f.preview(t, statement, "1", "1").AssertProblem(t, http.StatusUnprocessableEntity, "/problems/csv-mapping-required")
		f.assertNothingImported(t)
	})
}

// The same file again is refused with a pointer to the earlier import.
func TestTheSameFileTwiceIsRefused(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	first := f.confirm(t, statement, "2500.00")

	for name, res := range map[string]*apitest.Response{
		"upload":  f.upload(t, "", statement, "2500.00", ""),
		"preview": f.preview(t, statement, "2500.00", "2500.00"),
	} {
		var p struct {
			StatementImportID string `json:"statement_import_id"`
		}

		res.AssertProblem(t, http.StatusConflict, "/problems/duplicate-statement-import")

		if err := json.Unmarshal(res.Body, &p); err != nil || p.StatementImportID != first.StatementImportID {
			t.Errorf("%s: problem names %q, want the earlier import %s", name, p.StatementImportID, first.StatementImportID)
		}
	}

	if got := f.importList(t); len(got) != 1 {
		t.Errorf("statement imports = %+v, want one", got)
	}
}

// A file with any unreadable row is refused whole, listing every line and
// its reason, including amounts with more decimal places than EUR has.
func TestAFileWithUnreadableRowsIsRefusedWhole(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	bad := "Date;Description;Amount\n" +
		"01/01/2026;Fine;-1,00\n" +
		"32/01/2026;Bad date;-1,00\n" +
		"03/01/2026;Too precise;-1,005\n" +
		"04/01/2026;Not a number;abc\n"

	for name, res := range map[string]*apitest.Response{
		"preview": f.preview(t, bad, "10.00", "10.00"),
		"upload":  f.upload(t, "", bad, "10.00", "anything"),
	} {
		res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/unreadable-statement-file")

		var p rowsProblem
		if err := json.Unmarshal(res.Body, &p); err != nil {
			t.Fatal(err)
		}

		lines := []int{}
		for _, r := range p.Rows {
			lines = append(lines, r.Line)

			if r.Reason == "" {
				t.Errorf("%s: line %d has no reason", name, r.Line)
			}
		}

		if len(lines) != 3 || lines[0] != 3 || lines[1] != 4 || lines[2] != 5 {
			t.Errorf("%s: refused lines %v, want [3 4 5]", name, lines)
		}

		if !strings.Contains(p.Rows[1].Reason, "decimal places") {
			t.Errorf("%s: line 4 reason = %q, want the decimal places named", name, p.Rows[1].Reason)
		}
	}

	f.preview(t, "Date;Description;Amount\n", "1.00", "1.00").
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/empty-statement-file")

	f.assertNothingImported(t)
}

// Retrying an upload with the same Idempotency-Key returns the original
// result, even though the rebuilt form has another multipart boundary.
func TestRetryingAnUploadWithItsIdempotencyKeyReplaysIt(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	p := f.previewed(t, statement, "2500.00")

	first := f.upload(t, "upload-1", statement, "2500.00", p.PreviewID)
	if first.Status != http.StatusCreated {
		t.Fatalf("first upload: %d %s", first.Status, first.Body)
	}

	retry := f.upload(t, "upload-1", statement, "2500.00", p.PreviewID)
	if retry.Status != http.StatusCreated || string(retry.Body) != string(first.Body) {
		t.Errorf("retry = %d %s, want the original %s", retry.Status, retry.Body, first.Body)
	}

	f.upload(t, "upload-1", otherStatement, "2500.00", p.PreviewID).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/idempotency-key-mismatch")

	if got := f.importList(t); len(got) != 1 {
		t.Errorf("statement imports = %+v, want one", got)
	}

	if got := f.imported(t, ""); len(got.Items) != 3 {
		t.Errorf("imported transactions = %d, want 3", len(got.Items))
	}
}

func TestListingPagesAndFiltersByBookingMonth(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	f.confirm(t, statement, "2500.00")

	// Imported transactions, two per page.
	first := f.imported(t, "?limit=2")
	if len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page = %+v, want two and a cursor", first)
	}

	second := f.imported(t, "?limit=2&cursor="+first.NextCursor)
	if len(second.Items) != 1 || second.NextCursor != "" || second.Items[0].Description != "Coffee" {
		t.Fatalf("second page = %+v, want the last one and no cursor", second)
	}

	// The Account's Transactions: the Opening balance is booked in January.
	if got := f.transactions(t, "?booking_month=2026-02"); len(got.Items) != 0 {
		t.Errorf("February = %+v, want nothing", got.Items)
	}

	if got := f.transactions(t, "?booking_month=2026-01&limit=1"); len(got.Items) != 1 || got.NextCursor != "" {
		t.Errorf("January = %+v, want the Opening balance alone, and no next page", got)
	}

	if got := f.transactions(t, ""); len(got.Items) != 1 {
		t.Errorf("every month = %+v, want the Opening balance", got.Items)
	}

	f.app.Get(t, f.accountPath()+"/transactions?cursor=not-a-cursor", f.token).
		AssertProblem(t, http.StatusBadRequest, "/problems/invalid-cursor")
	f.app.Get(t, f.accountPath()+"/imported-transactions?cursor=not-a-cursor", f.token).
		AssertProblem(t, http.StatusBadRequest, "/problems/invalid-cursor")
	f.app.Get(t, f.accountPath()+"/transactions?booking_month=2026-13", f.token).
		AssertProblem(t, http.StatusBadRequest, "about:blank")
}

func TestAFirstImportSurvivesARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newImportFixture(t, app, "alice")

	f.confirm(t, statement, "2500.00")

	before := []any{f.getAccount(t), f.transactions(t, ""), f.imported(t, ""), f.importList(t)}

	app.Rebuild(t)

	after := []any{f.getAccount(t), f.transactions(t, ""), f.imported(t, ""), f.importList(t)}
	if !sameJSON(t, after, before) {
		t.Errorf("after rebuild:\n  %+v\nbefore:\n  %+v", after, before)
	}
}

func TestAnotherHouseholdsImportsAre404(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := newImportFixture(t, app, "alice")
	bob := newImportFixture(t, app, "bob")

	alice.confirm(t, statement, "2500.00")

	// Bob through Alice's Household, and through his own with her IDs.
	for _, f := range []importFixture{
		{fixture: fixture{app: app, token: bob.token, household: alice.household, account: alice.account}, connection: alice.connection},
		{fixture: fixture{app: app, token: bob.token, household: bob.household, account: alice.account}, connection: alice.connection},
	} {
		f.preview(t, otherStatement, "1.00", "1.00").AssertProblem(t, http.StatusNotFound, "about:blank")
		f.upload(t, "", otherStatement, "1.00", "x").AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Get(t, f.imports(), f.token).AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Get(t, f.accountPath()+"/transactions", f.token).AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Get(t, f.accountPath()+"/imported-transactions", f.token).AssertProblem(t, http.StatusNotFound, "about:blank")
	}

	if got := alice.importList(t); len(got) != 1 {
		t.Errorf("Alice's imports = %+v, want hers alone", got)
	}

	bob.assertNothingImported(t)
}
