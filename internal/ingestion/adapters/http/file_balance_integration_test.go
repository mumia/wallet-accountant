//go:build integration

package http_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// balanceStatement is statement with a running balance, oldest first: its
// file's balance is 2389.51 on line 5, and the row before the start date
// still counts for it. Its Opening balance is 2389.51 - 1989.50 = 400.01.
const balanceStatement = "Date;Description;Amount;Balance\n" +
	"31/12/2025;Before the start date;-99,99;400,01\n" +
	"01/01/2026;Rent;-1.000,00;-599,99\n" +
	"15/01/2026;Salary;3.000,00;2.400,01\n" +
	"02/02/2026;Coffee;-10,50;2.389,51\n"

// unbalancedStatement's balance column adds up from neither end.
const unbalancedStatement = "Date;Description;Amount;Balance\n" +
	"01/01/2026;Rent;-1.000,00;-1.000,00\n" +
	"15/01/2026;Salary;3.000,00;5,00\n"

// withBalanceColumn is signed() reading the running balance from the
// "Balance" column.
func withBalanceColumn() mapping {
	m := signed()
	columns := maps.Clone(m["columns"].(map[string]any))
	columns["balance"] = map[string]any{"header": "Balance"}

	return with(m, mapping{"columns": columns})
}

type fileBalanceProblem struct {
	FileBalance string `json:"file_balance"`
	Line        int    `json:"line"`
}

// newBalanceFixture is an import fixture whose Connection's mapping has a
// balance column.
func newBalanceFixture(t *testing.T) importFixture {
	t.Helper()

	f := newImportFixture(t, apitest.Start(t), "alice")

	var saved connection
	f.app.Put(t, f.connections()+"/"+f.connection+"/csv-mapping", f.token, withBalanceColumn()).
		AssertJSON(t, http.StatusOK, &saved)

	if c, ok := saved.CSVMapping["columns"].(map[string]any); !ok || c["balance"] == nil {
		t.Fatalf("saved mapping = %+v, want its balance column", saved.CSVMapping)
	}

	return f
}

// previewOnce previews a first import with its Export balance typed once.
func (f importFixture) previewOnce(t *testing.T, file, balance string) *apitest.Response {
	t.Helper()

	return f.app.PostForm(t, f.imports()+"/previews", f.token, "", apitest.Form{
		Fields: map[string]string{"export_balance": balance}, FileName: "statement.csv", File: []byte(file),
	})
}

// With a balance column, the file's balance tests the first Export balance,
// so it is asked once; a confirmation, if sent, is ignored.
func TestAFirstImportWithABalanceColumnAsksForTheExportBalanceOnce(t *testing.T) {
	t.Parallel()
	f := newBalanceFixture(t)

	var p preview
	f.previewOnce(t, balanceStatement, "2389.51").AssertJSON(t, http.StatusOK, &p)
	assertMoney(t, "opening_balance", p.OpeningBalance, "400.01")

	var ignored preview
	f.preview(t, balanceStatement, "2389.51", "1.00").AssertJSON(t, http.StatusOK, &ignored)

	if ignored.PreviewID != p.PreviewID {
		t.Errorf("preview with a confirmation = %+v, want the same preview", ignored)
	}

	var si statementImport
	f.upload(t, "", balanceStatement, "2389.51", p.PreviewID).AssertJSON(t, http.StatusCreated, &si)

	if si.OpeningBalance == nil || si.RowsImported != 3 {
		t.Fatalf("statement import = %+v, want 3 rows and an Opening balance", si)
	}

	assertMoney(t, "opening_balance", *si.OpeningBalance, "400.01")
}

// Without a balance column nothing changes: a missing confirmation is
// refused as entries that differ.
func TestAFirstImportWithoutABalanceColumnStillNeedsTheConfirmation(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	f.previewOnce(t, statement, "2500.00").
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/export-balance-mismatch")
	f.assertNothingImported(t)
}

func TestAnExportBalanceThatIsNotTheFilesBalanceIsRefused(t *testing.T) {
	t.Parallel()
	f := newBalanceFixture(t)

	for name, res := range map[string]*apitest.Response{
		"preview": f.preview(t, balanceStatement, "1000.00", "1000.00"),
		"upload":  f.upload(t, "", balanceStatement, "1000.00", ""),
	} {
		p := res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/export-balance-differs-from-file")

		var got fileBalanceProblem
		if err := json.Unmarshal(res.Body, &got); err != nil {
			t.Fatal(err)
		}

		if got.FileBalance != "2389.51" || got.Line != 5 {
			t.Errorf("%s: problem = %+v, want file_balance 2389.51 on line 5", name, got)
		}

		if want := "The Export balance, 1000.00 EUR, isn't the file's balance, 2389.51 EUR (line 5)."; !strings.HasPrefix(p.Detail, want) {
			t.Errorf("%s: detail = %q, want it to start %q", name, p.Detail, want)
		}
	}

	f.assertNothingImported(t)
}

func TestAFileWhoseBalanceColumnDoesNotAddUpIsRefused(t *testing.T) {
	t.Parallel()
	f := newBalanceFixture(t)

	for name, res := range map[string]*apitest.Response{
		"preview": f.previewOnce(t, unbalancedStatement, "5.00"),
		"upload":  f.upload(t, "", unbalancedStatement, "5.00", ""),
	} {
		p := res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/file-balance-unknown")

		if !strings.Contains(p.Detail, "doesn't add up from one end of the file to the other") {
			t.Errorf("%s: detail = %q, want the reason", name, p.Detail)
		}
	}

	f.assertNothingImported(t)
}

// A later import is checked against the file too, before its duplicate-file
// check and its Balance check.
func TestALaterImportChecksItsExportBalanceAgainstTheFile(t *testing.T) {
	t.Parallel()
	f := newBalanceFixture(t)

	var p preview
	f.previewOnce(t, balanceStatement, "2389.51").AssertJSON(t, http.StatusOK, &p)
	f.upload(t, "", balanceStatement, "2389.51", p.PreviewID).AssertJSON(t, http.StatusCreated, &statementImport{})

	// The same file with a wrong Export balance: the file's balance is
	// checked before the duplicate-file check.
	f.upload(t, "", balanceStatement, "2000.00", "").
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/export-balance-differs-from-file")
	f.upload(t, "", balanceStatement, "2389.51", "").
		AssertProblem(t, http.StatusConflict, "/problems/duplicate-statement-import")

	later := "Date;Description;Amount;Balance\n03/02/2026;Tea;-2,00;2.387,51\n"

	f.upload(t, "", later, "2387.50", "").
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/export-balance-differs-from-file")

	var si statementImport
	f.upload(t, "", later, "2387.51", "").AssertJSON(t, http.StatusCreated, &si)

	if si.BalanceCheck != "PASSED" {
		t.Errorf("statement import = %+v, want its Balance check passed", si)
	}
}

type balanceMappingTest struct {
	Rows []struct {
		Line    int    `json:"line"`
		Balance *money `json:"balance"`
	} `json:"rows"`
	FileBalance *struct {
		Amount money `json:"amount"`
		Line   int   `json:"line"`
	} `json:"file_balance"`
	FileBalanceProblem *string `json:"file_balance_problem"`
}

func TestTestingAMappingShowsEachRowsBalanceAndTheFilesBalance(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	var got balanceMappingTest
	f.testMapping(t, withBalanceColumn(), balanceStatement).AssertJSON(t, http.StatusOK, &got)

	if len(got.Rows) != 4 || got.Rows[0].Balance == nil || got.Rows[3].Balance == nil {
		t.Fatalf("rows = %+v, want 4 with balances", got.Rows)
	}

	assertMoney(t, "first row's balance", *got.Rows[0].Balance, "400.01")
	assertMoney(t, "last row's balance", *got.Rows[3].Balance, "2389.51")

	if got.FileBalance == nil || got.FileBalance.Line != 5 || got.FileBalanceProblem != nil {
		t.Fatalf("mapping test = %+v, want the file's balance on line 5", got)
	}

	assertMoney(t, "file_balance", got.FileBalance.Amount, "2389.51")

	var unbalanced balanceMappingTest
	f.testMapping(t, withBalanceColumn(), unbalancedStatement).AssertJSON(t, http.StatusOK, &unbalanced)

	if unbalanced.FileBalance != nil || unbalanced.FileBalanceProblem == nil ||
		!strings.Contains(*unbalanced.FileBalanceProblem, "doesn't add up") {
		t.Errorf("mapping test = %+v, want only file_balance_problem", unbalanced)
	}

	var plain balanceMappingTest
	f.testMapping(t, signed(), statement).AssertJSON(t, http.StatusOK, &plain)

	if plain.FileBalance != nil || plain.FileBalanceProblem != nil || plain.Rows[0].Balance != nil {
		t.Errorf("mapping test = %+v, want no balances without a balance column", plain)
	}
}
