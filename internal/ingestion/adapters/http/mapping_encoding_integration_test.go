//go:build integration

package http_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

func TestAMappingSavesItsEncoding(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	c := f.create(t, map[string]any{"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT"})

	for name, tc := range map[string]struct {
		mapping mapping
		want    string
	}{
		"Windows-1252": {with(signed(), mapping{"encoding": "WINDOWS_1252"}), "WINDOWS_1252"},
		"UTF-8":        {with(signed(), mapping{"encoding": "UTF_8"}), "UTF_8"},
		"absent":       {with(signed(), mapping{"encoding": nil}), "UTF_8"},
	} {
		t.Run(name, func(t *testing.T) {
			var saved connection
			f.app.Put(t, f.mappingOf(c), f.token, tc.mapping).AssertJSON(t, http.StatusOK, &saved)

			if saved.CSVMapping["encoding"] != tc.want {
				t.Errorf("saved encoding = %v, want %s", saved.CSVMapping["encoding"], tc.want)
			}

			if got := f.list(t); len(got) != 1 || got[0].CSVMapping["encoding"] != tc.want {
				t.Errorf("GET connections = %+v, want encoding %s", got, tc.want)
			}
		})
	}

	t.Run("unknown", func(t *testing.T) {
		p := f.app.Put(t, f.mappingOf(c), f.token, with(signed(), mapping{"encoding": "LATIN_9"})).
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-csv-mapping")

		if !strings.Contains(p.Detail, "encoding") {
			t.Errorf("detail = %q, want it to mention the encoding", p.Detail)
		}
	})
}

// windows1252Statement is written as a bank exporting Windows-1252 writes
// it: "é" is the byte 0xE9, "€" 0x80 and "è" 0xE8.
const windows1252Statement = "Date;Description;Amount\n" +
	"01/02/2026;Caf\xe9 Proc\xe8s;-3,50\n" +
	"02/02/2026;Transfer 100\x80;-100,00\n"

func TestImportingAWindows1252File(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	f.app.Put(t, f.connections()+"/"+f.connection+"/csv-mapping", f.token, with(signed(), mapping{"encoding": "WINDOWS_1252"})).
		AssertJSON(t, http.StatusOK, &connection{})

	si := f.confirm(t, windows1252Statement, "896.50")
	if si.RowsImported != 2 {
		t.Errorf("statement import = %+v, want 2 rows imported", si)
	}

	got := map[string]string{}
	for _, it := range f.imported(t, "").Items {
		got[it.Description] = string(it.Amount.Amount)
	}

	want := map[string]string{"Café Procès": "-3.50", "Transfer 100€": "-100.00"}
	if len(got) != len(want) || got["Café Procès"] != want["Café Procès"] || got["Transfer 100€"] != want["Transfer 100€"] {
		t.Errorf("Imported transactions = %v, want %v", got, want)
	}
}
