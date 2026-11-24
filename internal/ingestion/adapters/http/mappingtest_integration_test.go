//go:build integration

package http_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type parsedRow struct {
	Line              int     `json:"line"`
	BookingDate       string  `json:"booking_date"`
	TransactionDate   string  `json:"transaction_date"`
	ValueDate         *string `json:"value_date"`
	Description       string  `json:"description"`
	Payee             *string `json:"payee"`
	BankTransactionID *string `json:"bank_transaction_id"`
	Amount            money   `json:"amount"`
}

type mappingTest struct {
	Rows             []parsedRow `json:"rows"`
	LastRows         []parsedRow `json:"last_rows"`
	ReadableRowCount int         `json:"readable_row_count"`
	UnreadableRows   []struct {
		Line   int    `json:"line"`
		Reason string `json:"reason"`
	} `json:"unreadable_rows"`
}

func (f fixture) mappingTests() string {
	return "/api/households/" + f.household + "/accounts/" + f.account + "/csv-mapping-tests"
}

// testMapping posts the mapping and the file to the mapping test, checking
// that it records nothing.
func (f fixture) testMapping(t *testing.T, m mapping, file string) *apitest.Response {
	t.Helper()

	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	before := f.app.EventCount(t)

	res := f.app.PostForm(t, f.mappingTests(), f.token, "", apitest.Form{
		Fields: map[string]string{"csv_mapping": string(raw)}, FileName: "sample.csv", File: []byte(file),
	})

	if after := f.app.EventCount(t); after != before {
		t.Errorf("the mapping test recorded %d events, want none", after-before)
	}

	return res
}

func TestTestingAMappingOnAGoodFile(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	// The Account starts on 1 January 2026; December 2025 rows are shown
	// all the same.
	var file strings.Builder

	file.WriteString("Date;Description;Amount\n")

	for day := 20; day <= 31; day++ {
		fmt.Fprintf(&file, "%02d/12/2025;Row %d;-%d,5\n", day, day, day)
	}

	var got mappingTest
	f.testMapping(t, signed(), file.String()).AssertJSON(t, http.StatusOK, &got)

	if got.ReadableRowCount != 12 || len(got.Rows) != 10 || len(got.UnreadableRows) != 0 {
		t.Fatalf("mapping test = %d rows of %d readable, %d unreadable; want 10 of 12, none",
			len(got.Rows), got.ReadableRowCount, len(got.UnreadableRows))
	}

	first := got.Rows[0]
	if first.Line != 2 || first.BookingDate != "2025-12-20" || first.TransactionDate != "2025-12-20" ||
		first.Description != "Row 20" || first.Amount != (money{Amount: "-20.50", Currency: "EUR"}) ||
		first.ValueDate != nil || first.Payee != nil || first.BankTransactionID != nil {
		t.Errorf("first row = %+v", first)
	}

	if last := got.Rows[9]; last.Line != 11 || last.Description != "Row 29" {
		t.Errorf("tenth row = %+v, want line 11, Row 29", last)
	}
}

func TestTestingAMappingWithEveryOptionalField(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	var got mappingTest
	f.testMapping(t, debitCredit(), "Bank export\nAccount 123\n"+
		"2026-02-01,Coffee,1.50,,2026-01-31,2026-02-02,Café Central,TX-1\n").
		AssertJSON(t, http.StatusOK, &got)

	if len(got.Rows) != 1 {
		t.Fatalf("rows = %+v, want one", got.Rows)
	}

	r := got.Rows[0]
	if r.Line != 3 || r.TransactionDate != "2026-01-31" || r.ValueDate == nil || *r.ValueDate != "2026-02-02" ||
		r.Payee == nil || *r.Payee != "Café Central" || r.BankTransactionID == nil || *r.BankTransactionID != "TX-1" ||
		r.Amount.Amount != "-1.50" {
		t.Errorf("row = %+v", r)
	}
}

func TestTestingAMappingOnAMixedFile(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	var got mappingTest
	f.testMapping(t, signed(), "Date;Description;Amount\n"+
		"01/02/2026;Coffee;-1,50\n"+
		"2026-02-02;Bad date;-1,00\n"+
		"03/02/2026;Salary;2.000,00\n"+
		"04/02/2026;Too precise;-1,001\n").
		AssertJSON(t, http.StatusOK, &got)

	if got.ReadableRowCount != 2 || len(got.Rows) != 2 || got.Rows[0].Line != 2 || got.Rows[1].Line != 4 {
		t.Errorf("readable rows = %+v (count %d), want lines 2 and 4", got.Rows, got.ReadableRowCount)
	}

	if len(got.UnreadableRows) != 2 || got.UnreadableRows[0].Line != 3 || got.UnreadableRows[1].Line != 5 ||
		got.UnreadableRows[0].Reason == "" {
		t.Errorf("unreadable rows = %+v, want lines 3 and 5 with reasons", got.UnreadableRows)
	}
}

func TestTestingAMappingOnAnUnreadableFile(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	var got mappingTest
	f.testMapping(t, signed(), "Date;Text;Amount\n01/02/2026;Coffee;-1,50\n").AssertJSON(t, http.StatusOK, &got)

	if got.ReadableRowCount != 0 || len(got.Rows) != 0 || len(got.UnreadableRows) != 1 ||
		got.UnreadableRows[0].Line != 1 || !strings.Contains(got.UnreadableRows[0].Reason, "Description") {
		t.Errorf("mapping test = %+v, want only line 1 unreadable for its missing Description column", got)
	}
}

func TestTestingAMappingIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	t.Run("an invalid mapping", func(t *testing.T) {
		p := f.testMapping(t, with(signed(), mapping{"date_format": "LATER", "encoding": "LATIN_9"}),
			"Date;Description;Amount\n01/02/2026;Coffee;-1,50\n").
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-csv-mapping")

		if !strings.Contains(p.Detail, "encoding") || !strings.Contains(p.Detail, "date") {
			t.Errorf("detail = %q, want every problem", p.Detail)
		}
	})

	t.Run("an empty file", func(t *testing.T) {
		f.testMapping(t, signed(), "Date;Description;Amount\n").
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/empty-statement-file")
	})

	t.Run("an unknown Account", func(t *testing.T) {
		other := f
		other.account = "0198a3c4-0000-7000-8000-000000000000"

		other.testMapping(t, signed(), "Date;Description;Amount\n01/02/2026;Coffee;-1,50\n").
			AssertProblem(t, http.StatusNotFound, "about:blank")
	})
}
