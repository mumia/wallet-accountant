//go:build integration

package http_test

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// footerStatement ends with two footer lines, as some banks write them.
const footerStatement = "Date;Description;Amount\n" +
	"01/02/2026;Coffee;-1,00\n" +
	"02/02/2026;Tea;-2,00\n" +
	"Total;;-3,00\n" +
	"Generated on 24/11/2026\n\n"

func TestAMappingSavesItsTrailingLinesToSkip(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	c := f.create(t, map[string]any{"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT"})

	for name, tc := range map[string]struct {
		mapping mapping
		want    float64
	}{
		"two":    {with(signed(), mapping{"skip_trailing_lines": 2}), 2},
		"absent": {with(signed(), mapping{"skip_trailing_lines": nil}), 0},
	} {
		t.Run(name, func(t *testing.T) {
			var saved connection
			f.app.Put(t, f.mappingOf(c), f.token, tc.mapping).AssertJSON(t, http.StatusOK, &saved)

			if saved.CSVMapping["skip_trailing_lines"] != tc.want {
				t.Errorf("saved skip_trailing_lines = %v, want %v", saved.CSVMapping["skip_trailing_lines"], tc.want)
			}

			if got := f.list(t); len(got) != 1 || got[0].CSVMapping["skip_trailing_lines"] != tc.want {
				t.Errorf("GET connections = %+v, want skip_trailing_lines %v", got, tc.want)
			}
		})
	}

	t.Run("negative", func(t *testing.T) {
		p := f.app.Put(t, f.mappingOf(c), f.token, with(signed(), mapping{"skip_trailing_lines": -1})).
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-csv-mapping")

		if !strings.Contains(p.Detail, "trailing lines") {
			t.Errorf("detail = %q, want it to mention the trailing lines", p.Detail)
		}
	})
}

func TestTestingAMappingSkipsTrailingLines(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	var got mappingTest
	f.testMapping(t, with(signed(), mapping{"skip_trailing_lines": 2}), footerStatement).AssertJSON(t, http.StatusOK, &got)

	if got.ReadableRowCount != 2 || len(got.Rows) != 2 || len(got.UnreadableRows) != 0 || len(got.LastRows) != 0 {
		t.Errorf("mapping test = %+v, want both rows read and no footer", got)
	}

	var unskipped mappingTest
	f.testMapping(t, signed(), footerStatement).AssertJSON(t, http.StatusOK, &unskipped)

	if len(unskipped.UnreadableRows) != 2 || unskipped.UnreadableRows[0].Line != 4 || unskipped.UnreadableRows[1].Line != 5 {
		t.Errorf("unreadable rows = %+v, want the footer at lines 4 and 5", unskipped.UnreadableRows)
	}

	f.testMapping(t, with(signed(), mapping{"skip_leading_lines": 1, "skip_trailing_lines": 4}), footerStatement).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/empty-statement-file")
}

func TestTestingAMappingShowsTheLastRows(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")

	for _, tc := range []struct {
		rows      int
		wantLines []int
	}{
		{rows: 5, wantLines: []int{}},
		{rows: 15, wantLines: []int{12, 13, 14, 15, 16}},
		{rows: 30, wantLines: []int{22, 23, 24, 25, 26, 27, 28, 29, 30, 31}},
	} {
		t.Run(fmt.Sprintf("%d rows", tc.rows), func(t *testing.T) {
			var file strings.Builder

			file.WriteString("Date;Description;Amount\n")

			for i := 1; i <= tc.rows; i++ {
				fmt.Fprintf(&file, "01/12/2025;Row %d;-%d,00\n", i, i)
			}

			var got mappingTest
			f.testMapping(t, signed(), file.String()).AssertJSON(t, http.StatusOK, &got)

			if got.LastRows == nil {
				t.Fatal("last_rows is missing")
			}

			lines := make([]int, 0, len(got.LastRows))
			for _, r := range got.LastRows {
				lines = append(lines, r.Line)
			}

			if !reflect.DeepEqual(lines, tc.wantLines) {
				t.Errorf("last rows are lines %v, want %v", lines, tc.wantLines)
			}

			if n := len(got.LastRows); n > 0 {
				last := got.LastRows[n-1]
				if last.Description != fmt.Sprintf("Row %d", tc.rows) ||
					string(last.Amount.Amount) != fmt.Sprintf("-%d.00", tc.rows) {
					t.Errorf("last row = %+v, want Row %d", last, tc.rows)
				}
			}

			if got.ReadableRowCount != tc.rows || len(got.Rows) != min(tc.rows, 10) {
				t.Errorf("mapping test = %d rows of %d readable", len(got.Rows), got.ReadableRowCount)
			}
		})
	}
}

func TestImportingAFileWithAFooter(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	f.preview(t, footerStatement, "997.00", "997.00").
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/unreadable-statement-file")

	f.app.Put(t, f.connections()+"/"+f.connection+"/csv-mapping", f.token, with(signed(), mapping{"skip_trailing_lines": 2})).
		AssertJSON(t, http.StatusOK, &connection{})

	si := f.confirm(t, footerStatement, "997.00")
	if si.RowsImported != 2 {
		t.Errorf("statement import = %+v, want 2 rows imported", si)
	}

	got := map[string]string{}
	for _, it := range f.imported(t, "").Items {
		got[it.Description] = string(it.Amount.Amount)
	}

	if want := map[string]string{"Coffee": "-1.00", "Tea": "-2.00"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Imported transactions = %v, want %v", got, want)
	}
}
