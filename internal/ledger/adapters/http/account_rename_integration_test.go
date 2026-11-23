//go:build integration

package http_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

func TestRenamingAnAccount(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	created := addAccount(t, app, token, household, map[string]string{
		"name": "Main", "kind": "CREDIT_CARD", "currency": "USD", "start_date": "2026-02-01",
	})
	path := "/api/households/" + household + "/accounts/" + created.AccountID

	var renamed account
	app.Patch(t, path, token, map[string]any{"name": "  Travel card "}).AssertJSON(t, http.StatusOK, &renamed)

	want := created
	want.Name = "Travel card"

	if renamed != want {
		t.Fatalf("renamed = %+v, want %+v: only the name changes", renamed, want)
	}

	var got accountList
	app.Get(t, "/api/households/"+household+"/accounts", token).AssertJSON(t, http.StatusOK, &got)

	if len(got.Items) != 1 || got.Items[0] != want {
		t.Errorf("GET accounts = %+v, want %+v", got.Items, want)
	}

	t.Run("to its own name", func(t *testing.T) {
		before := app.EventCount(t)

		var same account
		app.Patch(t, path, token, map[string]any{"name": "Travel card  "}).AssertJSON(t, http.StatusOK, &same)

		if same != want {
			t.Errorf("renamed = %+v, want %+v unchanged", same, want)
		}

		if after := app.EventCount(t); after != before {
			t.Errorf("events = %d, want %d: the same name records nothing", after, before)
		}
	})

	t.Run("survives a rebuild", func(t *testing.T) {
		app.Rebuild(t)

		var rebuilt account
		app.Get(t, path, token).AssertJSON(t, http.StatusOK, &rebuilt)

		if rebuilt != want {
			t.Errorf("after rebuild = %+v, want %+v", rebuilt, want)
		}
	})
}

func TestRenamingAnAccountIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	created := addAccount(t, app, token, household, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	})
	path := "/api/households/" + household + "/accounts/" + created.AccountID

	t.Run("a name of only spaces", func(t *testing.T) {
		p := app.Patch(t, path, token, map[string]any{"name": "   "}).
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-account")

		if p.Detail != "The name must not be blank." {
			t.Errorf("detail = %q", p.Detail)
		}
	})

	// The contract's minLength and maxLength refuse these before the
	// handler runs.
	for name, body := range map[string]map[string]any{
		"an empty name":   {"name": ""},
		"a too long name": {"name": strings.Repeat("x", 201)},
		"no name":         {},
	} {
		t.Run(name, func(t *testing.T) {
			app.Patch(t, path, token, body).AssertProblem(t, http.StatusBadRequest, "about:blank")
		})
	}

	t.Run("an unknown Account", func(t *testing.T) {
		app.Patch(t, "/api/households/"+household+"/accounts/"+household, token, map[string]any{"name": "Elsewhere"}).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	t.Run("another Household's Account", func(t *testing.T) {
		bob := app.Register(t, "https://issuer.example", "bob")
		bobs := newHousehold(t, app, bob)

		app.Patch(t, path, bob, map[string]any{"name": "Mine"}).AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Patch(t, "/api/households/"+bobs+"/accounts/"+created.AccountID, bob, map[string]any{"name": "Mine"}).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	var got account
	app.Get(t, path, token).AssertJSON(t, http.StatusOK, &got)

	if got != created {
		t.Errorf("account = %+v, want %+v unchanged", got, created)
	}
}
