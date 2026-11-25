//go:build integration

package http_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type money struct {
	Amount   json.Number `json:"amount"`
	Currency string      `json:"currency"`
}

type account struct {
	AccountID                    string `json:"account_id"`
	Name                         string `json:"name"`
	Kind                         string `json:"kind"`
	Currency                     string `json:"currency"`
	StartDate                    string `json:"start_date"`
	AwaitingFirstStatementImport bool   `json:"awaiting_first_statement_import"`
	Balance                      *money `json:"balance"`
}

type accountList struct {
	Items []account `json:"items"`
}

func newHousehold(t *testing.T, app *apitest.App, token string) string {
	t.Helper()

	var created struct {
		HouseholdID string `json:"household_id"`
	}

	app.Post(t, "/api/households", token, map[string]string{"name": "Home", "base_currency": "EUR"}).
		AssertJSON(t, http.StatusCreated, &created)

	return created.HouseholdID
}

func addAccount(t *testing.T, app *apitest.App, token, household string, body map[string]string) account {
	t.Helper()

	var created account
	app.Post(t, "/api/households/"+household+"/accounts", token, body).AssertJSON(t, http.StatusCreated, &created)

	return created
}

func TestAddingAnAccountAndOpeningIt(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)

	created := addAccount(t, app, token, household, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	})

	want := account{
		AccountID: created.AccountID, Name: "Main", Kind: "BANK_ACCOUNT", Currency: "EUR",
		StartDate: "2026-01-01", AwaitingFirstStatementImport: true,
	}
	if created.AccountID == "" || created != want {
		t.Fatalf("created = %+v, want %+v with an ID", created, want)
	}

	var got account
	app.Get(t, "/api/households/"+household+"/accounts/"+created.AccountID, token).AssertJSON(t, http.StatusOK, &got)

	if got != want {
		t.Errorf("GET account = %+v, want %+v", got, want)
	}
}

func TestAnAccountSurvivesARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	created := addAccount(t, app, token, household, map[string]string{
		"name": "Cold storage", "kind": "CRYPTO_WALLET", "currency": "BTC", "start_date": "2025-12-31",
	})

	app.Rebuild(t)

	var got account
	app.Get(t, "/api/households/"+household+"/accounts/"+created.AccountID, token).AssertJSON(t, http.StatusOK, &got)

	if got != created {
		t.Errorf("after rebuild = %+v, want %+v", got, created)
	}
}

func TestAddingAnAccountTwiceWithOneIdempotencyKeyAddsOne(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	body := map[string]string{"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01"}

	var first, replay account
	app.PostWithKey(t, "/api/households/"+household+"/accounts", token, "k1", body).AssertJSON(t, http.StatusCreated, &first)
	app.PostWithKey(t, "/api/households/"+household+"/accounts", token, "k1", body).AssertJSON(t, http.StatusCreated, &replay)

	if replay != first {
		t.Errorf("replay = %+v, want %+v", replay, first)
	}

	var got accountList
	app.Get(t, "/api/households/"+household+"/accounts", token).AssertJSON(t, http.StatusOK, &got)

	if len(got.Items) != 1 {
		t.Errorf("accounts = %+v, want only the first", got.Items)
	}
}

func TestListingTheHouseholdsAccounts(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	other := newHousehold(t, app, token)

	var empty accountList
	app.Get(t, "/api/households/"+household+"/accounts", token).AssertJSON(t, http.StatusOK, &empty)

	if len(empty.Items) != 0 {
		t.Fatalf("before adding any: %+v, want none", empty.Items)
	}

	added := make([]account, 0, 4)
	for _, kind := range []string{"BANK_ACCOUNT", "CREDIT_CARD", "LOAN", "CRYPTO_WALLET"} {
		added = append(added, addAccount(t, app, token, household, map[string]string{
			"name": kind, "kind": kind, "currency": "EUR", "start_date": "2026-01-01",
		}))
	}

	addAccount(t, app, token, other, map[string]string{
		"name": "Elsewhere", "kind": "LOAN", "currency": "EUR", "start_date": "2026-01-01",
	})

	var got accountList
	app.Get(t, "/api/households/"+household+"/accounts", token).AssertJSON(t, http.StatusOK, &got)

	if !slices.Equal(got.Items, added) {
		t.Errorf("GET accounts = %+v, want %+v", got.Items, added)
	}
}

func TestAddingAnAccountIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)

	valid := map[string]string{"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01"}

	for name, change := range map[string]map[string]string{
		"unknown kind":     {"kind": "SAVINGS_JAR"},
		"lower-case kind":  {"kind": "bank_account"},
		"unknown currency": {"currency": "XYZ"},
		"blank name":       {"name": "   "},
	} {
		t.Run(name, func(t *testing.T) {
			body := maps.Clone(valid)
			maps.Copy(body, change)

			app.Post(t, "/api/households/"+household+"/accounts", token, body).
				AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-account")
		})
	}

	var got accountList
	app.Get(t, "/api/households/"+household+"/accounts", token).AssertJSON(t, http.StatusOK, &got)

	if len(got.Items) != 0 {
		t.Errorf("after refusals: %+v, want no Account", got.Items)
	}
}

func TestAnOpeningBalanceCannotBeTyped(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)

	res := app.Post(t, "/api/households/"+household+"/accounts", token, map[string]any{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
		"opening_balance": map[string]any{"amount": 100, "currency": "EUR"},
		"balance":         map[string]any{"amount": 100, "currency": "EUR"},
	})

	var created account
	res.AssertJSON(t, http.StatusCreated, &created)

	var got account
	app.Get(t, "/api/households/"+household+"/accounts/"+created.AccountID, token).AssertJSON(t, http.StatusOK, &got)

	if !got.AwaitingFirstStatementImport || got.Balance != nil {
		t.Errorf("account = %+v, want it awaiting its first Statement import with no balance", got)
	}
}

func TestAnotherHouseholdsAccountsAre404(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")
	alices := newHousehold(t, app, alice)
	bobs := newHousehold(t, app, bob)

	created := addAccount(t, app, alice, alices, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	})

	t.Run("listing", func(t *testing.T) {
		app.Get(t, "/api/households/"+alices+"/accounts", bob).AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	t.Run("opening", func(t *testing.T) {
		app.Get(t, "/api/households/"+alices+"/accounts/"+created.AccountID, bob).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	t.Run("opening through the caller's own Household", func(t *testing.T) {
		app.Get(t, "/api/households/"+bobs+"/accounts/"+created.AccountID, bob).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	t.Run("adding", func(t *testing.T) {
		app.Post(t, "/api/households/"+alices+"/accounts", bob, map[string]string{
			"name": "Sneaky", "kind": "LOAN", "currency": "EUR", "start_date": "2026-01-01",
		}).AssertProblem(t, http.StatusNotFound, "about:blank")
	})

	t.Run("an unknown Account", func(t *testing.T) {
		app.Get(t, "/api/households/"+alices+"/accounts/"+bobs, alice).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	})
}
