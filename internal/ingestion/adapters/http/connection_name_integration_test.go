//go:build integration

package http_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

func (f fixture) connection(c connection) string {
	return f.connections() + "/" + c.ConnectionID
}

func (f fixture) rename(t *testing.T, c connection, name string) connection {
	t.Helper()

	var renamed connection
	f.app.Patch(t, f.connection(c), f.token, map[string]any{"name": name}).AssertJSON(t, http.StatusOK, &renamed)

	return renamed
}

func TestAConnectionNameIsTakenWithinItsAccountIgnoringCase(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	first := f.create(t, map[string]any{"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT"})

	p := f.app.Post(t, f.connections(), f.token, map[string]any{"name": " STATEMENT FILE ", "kind": "STATEMENT_FILE_IMPORT"}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/connection-name-taken")

	if p.Title != "Connection name taken" || p.Detail != `This Account already has a Connection named "STATEMENT FILE".` {
		t.Errorf("problem = %+v, want the spec's copy naming the trimmed name", p)
	}

	if got := f.list(t); len(got) != 1 || !reflect.DeepEqual(got[0], first) {
		t.Errorf("connections = %+v, want only the first", got)
	}
}

func TestAnotherAccountMayUseTheSameConnectionName(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	other := f
	other.account = addAccount(t, f.app, f.token, f.household)

	f.create(t, map[string]any{"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT"})

	t.Run("on create", func(t *testing.T) {
		if got := other.create(t, map[string]any{"name": "statement file", "kind": "STATEMENT_FILE_IMPORT"}); got.Name != "statement file" {
			t.Errorf("created = %+v, want the name the first Account also uses", got)
		}
	})

	t.Run("on rename", func(t *testing.T) {
		card := f.create(t, map[string]any{"name": "Card statements", "kind": "STATEMENT_FILE_IMPORT"})
		other.create(t, map[string]any{"name": "Savings", "kind": "STATEMENT_FILE_IMPORT"})

		if got := f.rename(t, card, "SAVINGS"); got.Name != "SAVINGS" {
			t.Errorf("renamed = %+v, want the name the second Account also uses", got)
		}
	})
}

func TestRenamingAConnection(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newFixture(t, app, "alice")
	c := f.create(t, map[string]any{"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed()})

	renamed := f.rename(t, c, "  Card statements ")

	want := c
	want.Name = "Card statements"

	if !reflect.DeepEqual(renamed, want) {
		t.Fatalf("renamed = %+v, want %+v: only the name changes", renamed, want)
	}

	if got := f.list(t); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("GET connections = %+v, want %+v", got, want)
	}

	t.Run("only its case", func(t *testing.T) {
		if got := f.rename(t, c, "CARD statements"); got.Name != "CARD statements" {
			t.Errorf("renamed = %+v, want its own name in another case", got)
		}
	})

	t.Run("to its own name", func(t *testing.T) {
		if got := f.rename(t, c, " CARD statements"); got.Name != "CARD statements" || got.CSVMappingVersion != 1 {
			t.Errorf("renamed = %+v, want it unchanged", got)
		}
	})

	t.Run("survives a rebuild", func(t *testing.T) {
		app.Rebuild(t)

		if got := f.list(t); len(got) != 1 || got[0].Name != "CARD statements" {
			t.Errorf("after rebuild = %+v, want the last name", got)
		}
	})

	t.Run("frees the old name", func(t *testing.T) {
		if got := f.create(t, map[string]any{"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT"}); got.Name != "Statement file" {
			t.Errorf("created = %+v, want the first Connection's old name", got)
		}
	})
}

func TestRenamingAConnectionIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t, apitest.Start(t), "alice")
	c := f.create(t, map[string]any{"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT"})
	f.create(t, map[string]any{"name": "Card statements", "kind": "STATEMENT_FILE_IMPORT"})

	t.Run("a name another Connection has", func(t *testing.T) {
		p := f.app.Patch(t, f.connection(c), f.token, map[string]any{"name": "card STATEMENTS"}).
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/connection-name-taken")

		if p.Detail != `This Account already has a Connection named "card STATEMENTS".` {
			t.Errorf("detail = %q, want the spec's copy", p.Detail)
		}
	})

	t.Run("a name of only spaces", func(t *testing.T) {
		f.app.Patch(t, f.connection(c), f.token, map[string]any{"name": "   "}).
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-connection")
	})

	// The contract's minLength and maxLength refuse these before the
	// handler runs.
	for name, body := range map[string]map[string]any{
		"an empty name":   {"name": ""},
		"a too long name": {"name": strings.Repeat("x", 101)},
		"no name":         {},
	} {
		t.Run(name, func(t *testing.T) {
			f.app.Patch(t, f.connection(c), f.token, body).AssertProblem(t, http.StatusBadRequest, "about:blank")
		})
	}

	t.Run("through another Account", func(t *testing.T) {
		other := f
		other.account = addAccount(t, f.app, f.token, f.household)

		f.app.Patch(t, other.connection(c), f.token, map[string]any{"name": "Elsewhere"}).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	t.Run("an unknown Connection", func(t *testing.T) {
		f.app.Patch(t, f.connections()+"/"+f.account, f.token, map[string]any{"name": "Elsewhere"}).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	t.Run("in another Household", func(t *testing.T) {
		bob := newFixture(t, f.app, "bob")
		asBob := f
		asBob.token = bob.token

		f.app.Patch(t, asBob.connection(c), bob.token, map[string]any{"name": "Elsewhere"}).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	if got := f.list(t); len(got) != 2 || got[0].Name != "Statement file" {
		t.Errorf("after refusals: %+v, want the first Connection's name unchanged", got)
	}
}
