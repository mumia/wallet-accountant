//go:build integration

package http_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

func TestChangingYourDisplayName(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "subject-1")

	var before me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &before)

	var renamed me
	app.Patch(t, "/api/me", token, map[string]string{"display_name": "  Ana Lopes "}).
		AssertJSON(t, http.StatusOK, &renamed)

	want := before
	want.DisplayName = "Ana Lopes"

	if renamed != want {
		t.Errorf("PATCH /me = %+v, want %+v", renamed, want)
	}

	var got me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &got)

	if got != want {
		t.Errorf("GET /me = %+v, want %+v", got, want)
	}
}

func TestRenamingChangesOnlyTheCallersName(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	ana := app.Register(t, "https://issuer.example", "ana")
	bea := app.Register(t, "https://issuer.example", "bea")

	app.Patch(t, "/api/me", ana, map[string]string{"display_name": "Ana Lopes"}).AssertJSON(t, http.StatusOK, &me{})

	var other me
	app.Get(t, "/api/me", bea).AssertJSON(t, http.StatusOK, &other)

	if other.DisplayName != "Ana" {
		t.Errorf("the other User's display name = %q, want it unchanged", other.DisplayName)
	}
}

func TestAUserRegisteredBeforeDisplayNamesCanSetOne(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.RegisterWithoutDisplayName(t, "https://issuer.example", "subject-1")

	// Refold the User from its events, as a deploy of this change would.
	app.Rebuild(t)

	var legacy me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &legacy)

	if legacy.DisplayName != "" {
		t.Fatalf("display name = %q, want none before one is set", legacy.DisplayName)
	}

	var named me
	app.Patch(t, "/api/me", token, map[string]string{"display_name": "Ana"}).AssertJSON(t, http.StatusOK, &named)

	if named.DisplayName != "Ana" || named.UserID != legacy.UserID {
		t.Errorf("PATCH /me = %+v, want %s named Ana", named, legacy.UserID)
	}
}

func TestADisplayNameOfOnlySpacesIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "subject-1")
	events := app.EventCount(t)

	app.Patch(t, "/api/me", token, map[string]string{"display_name": "   "}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-user")

	if n := app.EventCount(t); n != events {
		t.Errorf("events = %d, want %d: a refused rename records nothing", n, events)
	}
}

func TestAMissingEmptyOrTooLongNewDisplayNameIsABadRequest(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "subject-1")

	for name, body := range map[string]map[string]string{
		"missing":  {},
		"empty":    {"display_name": ""},
		"too long": {"display_name": strings.Repeat("a", 65)},
	} {
		t.Run(name, func(t *testing.T) {
			app.Patch(t, "/api/me", token, body).AssertProblem(t, http.StatusBadRequest, "about:blank")
		})
	}
}

func TestRenamingToTheSameNameRecordsNothing(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "subject-1")
	events := app.EventCount(t)

	var got me
	app.Patch(t, "/api/me", token, map[string]string{"display_name": " Ana "}).AssertJSON(t, http.StatusOK, &got)

	if got.DisplayName != "Ana" {
		t.Errorf("display name = %q, want %q", got.DisplayName, "Ana")
	}

	if n := app.EventCount(t); n != events {
		t.Errorf("events = %d, want %d: the same name records nothing", n, events)
	}
}

func TestARebuildKeepsTheNewDisplayName(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "subject-1")

	app.Patch(t, "/api/me", token, map[string]string{"display_name": "Ana Lopes"}).AssertJSON(t, http.StatusOK, &me{})

	app.Rebuild(t)

	var got me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &got)

	if got.DisplayName != "Ana Lopes" {
		t.Errorf("display name after rebuild = %q, want %q", got.DisplayName, "Ana Lopes")
	}
}

func TestRenamingAsAnUnregisteredIdentityRequiresRegistration(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	app.Patch(t, "/api/me", token, map[string]string{"display_name": "Ana"}).
		AssertProblem(t, http.StatusForbidden, "/problems/registration-required")
}

func TestRenamingWithoutATokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	app.Patch(t, "/api/me", "", map[string]string{"display_name": "Ana"}).
		AssertProblem(t, http.StatusUnauthorized, "about:blank")
}
