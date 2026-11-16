//go:build integration

package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type household struct {
	HouseholdID string `json:"household_id"`
}

type category struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ParentID   string `json:"parent_id"`
}

type node struct {
	category

	Children []node `json:"children"`
}

type tree struct {
	Items []node `json:"items"`
}

// alice is a registered User with a Household.
func alice(t *testing.T) (*apitest.App, string, string) {
	t.Helper()

	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")

	return app, token, createHousehold(t, app, token, "Home")
}

func createHousehold(t *testing.T, app *apitest.App, token, name string) string {
	t.Helper()

	var created household
	app.Post(t, "/households", token, map[string]string{"name": name, "base_currency": "EUR"}).
		AssertJSON(t, http.StatusCreated, &created)

	return created.HouseholdID
}

func categoriesPath(household string) string { return "/households/" + household + "/categories" }

func create(t *testing.T, app *apitest.App, token, household string, body map[string]any) category {
	t.Helper()

	var created category
	app.Post(t, categoriesPath(household), token, body).AssertJSON(t, http.StatusCreated, &created)

	return created
}

func patch(t *testing.T, app *apitest.App, token, household, id string, body map[string]any) *apitest.Response {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	return app.Do(t, http.MethodPatch, categoriesPath(household)+"/"+id, token, bytes.NewReader(raw))
}

func list(t *testing.T, app *apitest.App, token, household string) tree {
	t.Helper()

	var got tree
	app.Get(t, categoriesPath(household), token).AssertJSON(t, http.StatusOK, &got)

	return got
}

// outline renders a tree as "Name (KIND)" lines indented by depth, and
// checks every node's parent_id against its place in the tree.
func outline(t *testing.T, nodes []node, parent, indent string) []string {
	t.Helper()

	var lines []string

	for _, n := range nodes {
		if n.ParentID != parent {
			t.Errorf("%s has parent_id %q, but is listed under %q", n.Name, n.ParentID, parent)
		}

		lines = append(lines, indent+n.Name+" ("+n.Kind+")")
		lines = append(lines, outline(t, n.Children, n.CategoryID, indent+"  ")...)
	}

	return lines
}

func assertTree(t *testing.T, got tree, want ...string) {
	t.Helper()

	if lines := outline(t, got.Items, "", ""); !slices.Equal(lines, want) {
		t.Errorf("category tree:\n  got  %q\n  want %q", lines, want)
	}
}

func TestBuildingTheCategoryTree(t *testing.T) {
	t.Parallel()
	app, token, home := alice(t)

	food := create(t, app, token, home, map[string]any{"name": "Food", "kind": "EXPENSE"})
	if food.Kind != "EXPENSE" || food.ParentID != "" || food.CategoryID == "" {
		t.Fatalf("created %+v, want an Expense root Category", food)
	}

	eatingOut := create(t, app, token, home, map[string]any{"name": "Eating out", "parent_id": food.CategoryID})
	if eatingOut.Kind != "EXPENSE" || eatingOut.ParentID != food.CategoryID {
		t.Fatalf("created %+v, want an Expense Category under Food", eatingOut)
	}

	create(t, app, token, home, map[string]any{"name": "Coffee", "kind": "EXPENSE", "parent_id": eatingOut.CategoryID})
	create(t, app, token, home, map[string]any{"name": "Groceries", "parent_id": food.CategoryID})
	create(t, app, token, home, map[string]any{"name": "Salary", "kind": "INCOME"})

	assertTree(t, list(t, app, token, home),
		"Food (EXPENSE)",
		"  Eating out (EXPENSE)",
		"    Coffee (EXPENSE)",
		"  Groceries (EXPENSE)",
		"Household transfer (INTERNAL)",
		"Opening balance (INTERNAL)",
		"Salary (INCOME)",
		"Uncategorised (INTERNAL)",
	)
}

func TestRenamingAndMovingACategory(t *testing.T) {
	t.Parallel()
	app, token, home := alice(t)

	food := create(t, app, token, home, map[string]any{"name": "Food", "kind": "EXPENSE"})
	eatingOut := create(t, app, token, home, map[string]any{"name": "Eating out", "parent_id": food.CategoryID})
	coffee := create(t, app, token, home, map[string]any{"name": "Coffee", "parent_id": eatingOut.CategoryID})

	var renamed category
	patch(t, app, token, home, coffee.CategoryID, map[string]any{"name": "Espresso"}).
		AssertJSON(t, http.StatusOK, &renamed)

	if want := (category{coffee.CategoryID, "Espresso", "EXPENSE", eatingOut.CategoryID}); renamed != want {
		t.Errorf("renamed = %+v, want %+v", renamed, want)
	}

	var moved category
	patch(t, app, token, home, coffee.CategoryID, map[string]any{"parent_id": food.CategoryID}).
		AssertJSON(t, http.StatusOK, &moved)

	if moved.ParentID != food.CategoryID || moved.Name != "Espresso" {
		t.Errorf("moved = %+v, want Espresso under Food", moved)
	}

	// Eating out moves to the root, with no name change: an explicit null.
	var toRoot category
	patch(t, app, token, home, eatingOut.CategoryID, map[string]any{"parent_id": nil}).
		AssertJSON(t, http.StatusOK, &toRoot)

	if toRoot.ParentID != "" || toRoot.Name != "Eating out" {
		t.Errorf("moved = %+v, want Eating out as a root Category", toRoot)
	}

	// Renaming and moving at once, where only the new name is free.
	create(t, app, token, home, map[string]any{"name": "Coffee", "parent_id": eatingOut.CategoryID})
	var tea category
	patch(t, app, token, home, coffee.CategoryID, map[string]any{"name": "Tea", "parent_id": eatingOut.CategoryID}).
		AssertJSON(t, http.StatusOK, &tea)

	if want := (category{coffee.CategoryID, "Tea", "EXPENSE", eatingOut.CategoryID}); tea != want {
		t.Errorf("renamed and moved = %+v, want %+v", tea, want)
	}

	assertTree(t, list(t, app, token, home),
		"Eating out (EXPENSE)",
		"  Coffee (EXPENSE)",
		"  Tea (EXPENSE)",
		"Food (EXPENSE)",
		"Household transfer (INTERNAL)",
		"Opening balance (INTERNAL)",
		"Uncategorised (INTERNAL)",
	)

	// An empty change changes nothing.
	var unchanged category
	patch(t, app, token, home, food.CategoryID, map[string]any{}).AssertJSON(t, http.StatusOK, &unchanged)

	if unchanged != food {
		t.Errorf("after an empty change: %+v, want %+v", unchanged, food)
	}
}

func internalCategory(t *testing.T, app *apitest.App, token, household, name string) category {
	t.Helper()

	for _, n := range list(t, app, token, household).Items {
		if n.Name == name {
			return n.category
		}
	}

	t.Fatalf("no Internal Category %q", name)

	return category{}
}

func TestCreateCategoryRefusals(t *testing.T) {
	t.Parallel()
	app, token, home := alice(t)

	food := create(t, app, token, home, map[string]any{"name": "Food", "kind": "EXPENSE"})
	uncategorised := internalCategory(t, app, token, home, "Uncategorised")
	before := list(t, app, token, home)

	for _, tc := range []struct {
		name   string
		body   map[string]any
		status int
		typ    string
	}{
		{"blank name", map[string]any{"name": "  ", "kind": "EXPENSE"}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"empty name", map[string]any{"name": "", "kind": "EXPENSE"}, http.StatusBadRequest, "about:blank"},
		{"root without a kind", map[string]any{"name": "Misc"}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"unknown kind", map[string]any{"name": "Misc", "kind": "EQUITY"}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"Internal kind", map[string]any{"name": "Suspense", "kind": "INTERNAL"}, http.StatusUnprocessableEntity, "/problems/internal-category"},
		{"Income under an Expense parent", map[string]any{"name": "Refunds", "kind": "INCOME", "parent_id": food.CategoryID}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"under an Internal Category", map[string]any{"name": "Later", "parent_id": uncategorised.CategoryID}, http.StatusUnprocessableEntity, "/problems/internal-category"},
		{"unknown parent", map[string]any{"name": "Coffee", "parent_id": "019a8b2c-0000-7000-8000-00000000ffff"}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"name taken by a sibling", map[string]any{"name": " food ", "kind": "INCOME"}, http.StatusConflict, "/problems/category-name-taken"},
		{"name taken by an Internal Category", map[string]any{"name": "Uncategorised", "kind": "EXPENSE"}, http.StatusConflict, "/problems/category-name-taken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.Post(t, categoriesPath(home), token, tc.body).AssertProblem(t, tc.status, tc.typ)
		})
	}

	if after := list(t, app, token, home); !slices.Equal(outline(t, after.Items, "", ""), outline(t, before.Items, "", "")) {
		t.Errorf("refused creations changed the tree: %+v", after.Items)
	}
}

func TestChangeCategoryRefusals(t *testing.T) {
	t.Parallel()
	app, token, home := alice(t)

	food := create(t, app, token, home, map[string]any{"name": "Food", "kind": "EXPENSE"})
	eatingOut := create(t, app, token, home, map[string]any{"name": "Eating out", "parent_id": food.CategoryID})
	coffee := create(t, app, token, home, map[string]any{"name": "Coffee", "parent_id": eatingOut.CategoryID})
	create(t, app, token, home, map[string]any{"name": "Coffee", "parent_id": food.CategoryID})
	create(t, app, token, home, map[string]any{"name": "Groceries", "parent_id": food.CategoryID})
	salary := create(t, app, token, home, map[string]any{"name": "Salary", "kind": "INCOME"})
	uncategorised := internalCategory(t, app, token, home, "Uncategorised")
	before := list(t, app, token, home)

	for _, tc := range []struct {
		name   string
		id     string
		body   map[string]any
		status int
		typ    string
	}{
		{"unknown Category", "019a8b2c-0000-7000-8000-00000000ffff", map[string]any{"name": "Tea"}, http.StatusNotFound, "about:blank"},
		{"blank name", coffee.CategoryID, map[string]any{"name": " "}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"renamed to a sibling's name", eatingOut.CategoryID, map[string]any{"name": "groceries"}, http.StatusConflict, "/problems/category-name-taken"},
		{"moved where its name is taken", coffee.CategoryID, map[string]any{"parent_id": food.CategoryID}, http.StatusConflict, "/problems/category-name-taken"},
		{"moved under another kind", coffee.CategoryID, map[string]any{"parent_id": salary.CategoryID}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"moved under itself", food.CategoryID, map[string]any{"parent_id": food.CategoryID}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"moved under its own sub-Category", food.CategoryID, map[string]any{"parent_id": eatingOut.CategoryID}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"moved under an unknown parent", coffee.CategoryID, map[string]any{"parent_id": "019a8b2c-0000-7000-8000-00000000ffff"}, http.StatusUnprocessableEntity, "/problems/invalid-category"},
		{"moved under an Internal Category", coffee.CategoryID, map[string]any{"parent_id": uncategorised.CategoryID}, http.StatusUnprocessableEntity, "/problems/internal-category"},
		{"Internal Category renamed", uncategorised.CategoryID, map[string]any{"name": "Suspense"}, http.StatusUnprocessableEntity, "/problems/internal-category"},
		{"Internal Category moved", uncategorised.CategoryID, map[string]any{"parent_id": food.CategoryID}, http.StatusUnprocessableEntity, "/problems/internal-category"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patch(t, app, token, home, tc.id, tc.body).AssertProblem(t, tc.status, tc.typ)
		})
	}

	if after := list(t, app, token, home); !slices.Equal(outline(t, after.Items, "", ""), outline(t, before.Items, "", "")) {
		t.Errorf("refused changes changed the tree: %+v", after.Items)
	}
}

func TestAnotherHouseholdGets404ForEveryCategoryRoute(t *testing.T) {
	t.Parallel()
	app, alicesToken, alices := alice(t)
	food := create(t, app, alicesToken, alices, map[string]any{"name": "Food", "kind": "EXPENSE"})

	bobsToken := app.Register(t, "https://issuer.example", "bob")
	bobs := createHousehold(t, app, bobsToken, "Bob's")
	stranger := app.TokenFor("https://issuer.example", "stranger")

	for _, token := range []string{bobsToken, stranger} {
		app.Get(t, categoriesPath(alices), token).AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Post(t, categoriesPath(alices), token, map[string]any{"name": "Mine", "kind": "EXPENSE"}).
			AssertProblem(t, http.StatusNotFound, "about:blank")
		patch(t, app, token, alices, food.CategoryID, map[string]any{"name": "Mine"}).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	}

	// Alice's Category isn't in Bob's Household either, even by its ID.
	patch(t, app, bobsToken, bobs, food.CategoryID, map[string]any{"name": "Mine"}).
		AssertProblem(t, http.StatusNotFound, "about:blank")
	app.Post(t, categoriesPath(bobs), bobsToken, map[string]any{"name": "Mine", "parent_id": food.CategoryID}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-category")

	assertTree(t, list(t, app, alicesToken, alices),
		"Food (EXPENSE)",
		"Household transfer (INTERNAL)",
		"Opening balance (INTERNAL)",
		"Uncategorised (INTERNAL)",
	)
}

func TestConcurrentCreatesOfOneNameLetOnlyOneThrough(t *testing.T) {
	t.Parallel()
	app, token, home := alice(t)

	const n = 16

	var (
		wg       sync.WaitGroup
		statuses = make([]int, n)
	)

	for i := range n {
		wg.Go(func() {
			statuses[i] = app.Post(t, categoriesPath(home), token, map[string]any{"name": "Food", "kind": "EXPENSE"}).Status
		})
	}

	wg.Wait()

	slices.Sort(statuses)

	want := append([]int{http.StatusCreated}, slices.Repeat([]int{http.StatusConflict}, n-1)...)
	if !slices.Equal(statuses, want) {
		t.Errorf("statuses = %v, want one 201 and the rest 409", statuses)
	}
}

func TestCreatingACategoryIsIdempotent(t *testing.T) {
	t.Parallel()
	app, token, home := alice(t)

	body := map[string]any{"name": "Food", "kind": "EXPENSE"}

	var first, again category
	app.PostWithKey(t, categoriesPath(home), token, "create-food", body).AssertJSON(t, http.StatusCreated, &first)
	app.PostWithKey(t, categoriesPath(home), token, "create-food", body).AssertJSON(t, http.StatusCreated, &again)

	if again != first {
		t.Errorf("replayed %+v, want %+v", again, first)
	}

	assertTree(t, list(t, app, token, home),
		"Food (EXPENSE)",
		"Household transfer (INTERNAL)",
		"Opening balance (INTERNAL)",
		"Uncategorised (INTERNAL)",
	)
}

func TestRebuildRestoresTheCategoryTree(t *testing.T) {
	t.Parallel()
	app, token, home := alice(t)

	food := create(t, app, token, home, map[string]any{"name": "Food", "kind": "EXPENSE"})
	eatingOut := create(t, app, token, home, map[string]any{"name": "Eating out", "parent_id": food.CategoryID})
	coffee := create(t, app, token, home, map[string]any{"name": "Coffee", "parent_id": eatingOut.CategoryID})

	patch(t, app, token, home, coffee.CategoryID, map[string]any{"name": "Espresso", "parent_id": food.CategoryID}).
		AssertJSON(t, http.StatusOK, &category{})
	patch(t, app, token, home, eatingOut.CategoryID, map[string]any{"name": "Restaurants", "parent_id": nil}).
		AssertJSON(t, http.StatusOK, &category{})

	before := list(t, app, token, home)

	app.Rebuild(t)

	after := list(t, app, token, home)
	if got, want := outline(t, after.Items, "", ""), outline(t, before.Items, "", ""); !slices.Equal(got, want) {
		t.Errorf("tree after rebuild = %q, want %q", got, want)
	}

	assertTree(t, after,
		"Food (EXPENSE)",
		"  Espresso (EXPENSE)",
		"Household transfer (INTERNAL)",
		"Opening balance (INTERNAL)",
		"Restaurants (EXPENSE)",
		"Uncategorised (INTERNAL)",
	)
}
