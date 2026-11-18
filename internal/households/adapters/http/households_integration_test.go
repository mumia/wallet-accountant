//go:build integration

package http_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type household struct {
	HouseholdID  string `json:"household_id"`
	Name         string `json:"name"`
	BaseCurrency string `json:"base_currency"`
}

type householdList struct {
	Items []household `json:"items"`
}

type category struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ParentID   string `json:"parent_id"`
}

type categoryList struct {
	Items []category `json:"items"`
}

func createHousehold(t *testing.T, app *apitest.App, token, name string) household {
	t.Helper()

	var created household
	app.Post(t, "/api/households", token, map[string]string{"name": name, "base_currency": "EUR"}).
		AssertJSON(t, http.StatusCreated, &created)

	return created
}

func TestCreatingAHouseholdAndOpeningIt(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")

	var created household
	app.Post(t, "/api/households", token, map[string]string{"name": "Home", "base_currency": "EUR"}).
		AssertJSON(t, http.StatusCreated, &created)

	if created.HouseholdID == "" || created.Name != "Home" || created.BaseCurrency != "EUR" {
		t.Fatalf("created = %+v, want Home in EUR with an ID", created)
	}

	var got household
	app.Get(t, "/api/households/"+created.HouseholdID, token).AssertJSON(t, http.StatusOK, &got)

	if got != created {
		t.Errorf("GET /households/{id} = %+v, want %+v", got, created)
	}
}

func TestListingTheCallersHouseholds(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")

	var empty householdList
	app.Get(t, "/api/households", alice).AssertJSON(t, http.StatusOK, &empty)

	if len(empty.Items) != 0 {
		t.Fatalf("before creating any: %+v, want none", empty.Items)
	}

	home := createHousehold(t, app, alice, "Home")
	holiday := createHousehold(t, app, alice, "Holiday flat")
	createHousehold(t, app, bob, "Bob's")

	var got householdList
	app.Get(t, "/api/households", alice).AssertJSON(t, http.StatusOK, &got)

	if want := []household{home, holiday}; !slices.Equal(got.Items, want) {
		t.Errorf("GET /households = %+v, want %+v", got.Items, want)
	}
}

func TestANewHouseholdComesWithItsThreeInternalCategories(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, token, "Home")

	var got categoryList
	app.Get(t, "/api/households/"+home.HouseholdID+"/categories", token).AssertJSON(t, http.StatusOK, &got)

	names := make([]string, 0, len(got.Items))

	for _, c := range got.Items {
		if c.Kind != "INTERNAL" || c.ParentID != "" || c.CategoryID == "" {
			t.Errorf("category %+v, want an Internal root Category", c)
		}

		names = append(names, c.Name)
	}

	slices.Sort(names)

	if want := []string{"Household transfer", "Opening balance", "Uncategorised"}; !slices.Equal(names, want) {
		t.Errorf("categories = %v, want %v", names, want)
	}
}

func TestEachUserGets404ForTheOtherUsersHousehold(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")
	alices := createHousehold(t, app, alice, "Alice's")
	bobs := createHousehold(t, app, bob, "Bob's")

	for _, tc := range []struct {
		token string
		other household
	}{
		{alice, bobs},
		{bob, alices},
	} {
		for _, path := range []string{
			"/api/households/" + tc.other.HouseholdID,
			"/api/households/" + tc.other.HouseholdID + "/categories",
		} {
			app.Get(t, path, tc.token).AssertProblem(t, http.StatusNotFound, "about:blank")
		}
	}

	for _, tc := range []struct {
		token string
		own   household
	}{
		{alice, alices},
		{bob, bobs},
	} {
		var list householdList
		app.Get(t, "/api/households", tc.token).AssertJSON(t, http.StatusOK, &list)

		if len(list.Items) != 1 || list.Items[0] != tc.own {
			t.Errorf("listed %+v, want only %+v", list.Items, tc.own)
		}
	}
}

func TestAHouseholdThatDoesNotExistIs404LikeAnotherHouseholds(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")

	app.Get(t, "/api/households/019a8b2c-0000-7000-8000-00000000ffff", token).
		AssertProblem(t, http.StatusNotFound, "about:blank")
}

func TestUnregisteredCallers(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")
	stranger := app.TokenFor("https://issuer.example", "stranger")

	app.Get(t, "/api/households", stranger).AssertProblem(t, http.StatusForbidden, "/problems/registration-required")
	app.Post(t, "/api/households", stranger, map[string]string{"name": "Mine", "base_currency": "EUR"}).
		AssertProblem(t, http.StatusForbidden, "/problems/registration-required")

	for _, path := range []string{
		"/api/households/" + home.HouseholdID,
		"/api/households/" + home.HouseholdID + "/categories",
	} {
		app.Get(t, path, stranger).AssertProblem(t, http.StatusNotFound, "about:blank")
	}
}

func TestCreateHouseholdRefusals(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")

	for _, tc := range []struct {
		name   string
		body   map[string]string
		status int
		typ    string
	}{
		{"blank name", map[string]string{"name": "   ", "base_currency": "EUR"}, http.StatusUnprocessableEntity, "/problems/invalid-household"},
		{"unknown currency", map[string]string{"name": "Home", "base_currency": "XYZ"}, http.StatusUnprocessableEntity, "/problems/invalid-household"},
		{"malformed currency", map[string]string{"name": "Home", "base_currency": "euro"}, http.StatusBadRequest, "about:blank"},
		{"missing name", map[string]string{"base_currency": "EUR"}, http.StatusBadRequest, "about:blank"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.Post(t, "/api/households", token, tc.body).AssertProblem(t, tc.status, tc.typ)
		})
	}

	var none householdList
	app.Get(t, "/api/households", token).AssertJSON(t, http.StatusOK, &none)

	if len(none.Items) != 0 {
		t.Errorf("refused creations left Households: %+v", none.Items)
	}
}

func TestRebuildRestoresHouseholdsAndTheirCategories(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, token, "Home")

	var before categoryList
	app.Get(t, "/api/households/"+home.HouseholdID+"/categories", token).AssertJSON(t, http.StatusOK, &before)

	app.Rebuild(t)

	var list householdList
	app.Get(t, "/api/households", token).AssertJSON(t, http.StatusOK, &list)

	if len(list.Items) != 1 || list.Items[0] != home {
		t.Errorf("after rebuild: %+v, want only %+v", list.Items, home)
	}

	var after categoryList
	app.Get(t, "/api/households/"+home.HouseholdID+"/categories", token).AssertJSON(t, http.StatusOK, &after)

	if !slices.Equal(after.Items, before.Items) {
		t.Errorf("categories after rebuild = %+v, want %+v", after.Items, before.Items)
	}
}
