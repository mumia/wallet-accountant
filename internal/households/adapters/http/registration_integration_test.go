//go:build integration

package http_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
	"github.com/mumia/wallet-accountant/internal/households/domain"
)

type me struct {
	UserID                  string `json:"user_id"`
	IsInstanceAdministrator bool   `json:"is_instance_administrator"`
	DisplayName             string `json:"display_name,omitempty"`
}

func TestRedeemingASeededInvitationRegistersTheCaller(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	code := app.SeedInvitation(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	res := app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": code, "display_name": "Ana"})

	var registered me
	res.AssertJSON(t, http.StatusCreated, &registered)

	if registered.UserID == "" || registered.IsInstanceAdministrator || registered.DisplayName != "Ana" {
		t.Errorf("registered = %+v, want a User named Ana who is not Instance administrator", registered)
	}

	var got me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &got)

	if got != registered {
		t.Errorf("GET /me = %+v, want %+v", got, registered)
	}
}

func TestEveryInvalidCodeGetsTheSameGenericError(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	expired := app.SeedInvitationWith(t, domain.RoleUser, time.Now().Add(-time.Minute))

	used := app.SeedInvitation(t)
	app.Post(t, "/api/registrations", app.TokenFor("https://issuer.example", "first"),
		map[string]string{"invitation_code": used, "display_name": "Ana"}).AssertJSON(t, http.StatusCreated, &me{})

	superseded, err := app.RunBootstrap(t)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := app.RunBootstrap(t); err != nil {
		t.Fatal(err)
	}

	var details []apitest.ProblemBody

	for name, code := range map[string]string{
		"unknown":    "not-a-code-anyone-issued",
		"expired":    expired,
		"superseded": superseded,
		"used":       used,
	} {
		token := app.TokenFor("https://issuer.example", "caller-with-"+name+"-code")

		p := app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": code, "display_name": "Ana"}).
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-invitation")
		details = append(details, p)

		app.Get(t, "/api/me", token).AssertProblem(t, http.StatusForbidden, "/problems/registration-required")
	}

	for _, p := range details[1:] {
		if p != details[0] {
			t.Errorf("problems differ, so codes can be probed: %+v vs %+v", p, details[0])
		}
	}
}

func TestAnEmptyCodeIsABadRequest(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": "", "display_name": "Ana"}).
		AssertProblem(t, http.StatusBadRequest, "about:blank")
}

func TestTheDisplayNameIsTrimmed(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	var registered me
	app.Post(t, "/api/registrations", token,
		map[string]string{"invitation_code": app.SeedInvitation(t), "display_name": "  Ana Lopes "}).
		AssertJSON(t, http.StatusCreated, &registered)

	if registered.DisplayName != "Ana Lopes" {
		t.Errorf("display name = %q, want %q", registered.DisplayName, "Ana Lopes")
	}
}

func TestADisplayNameOfOnlySpacesIsRefusedAndTheCodeStaysUnused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	code := app.SeedInvitation(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": code, "display_name": "   "}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-user")

	app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": code, "display_name": "Ana"}).
		AssertJSON(t, http.StatusCreated, &me{})
}

func TestAMissingEmptyOrTooLongDisplayNameIsABadRequest(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	for name, body := range map[string]map[string]string{
		"missing":  {"invitation_code": "code"},
		"empty":    {"invitation_code": "code", "display_name": ""},
		"too long": {"invitation_code": "code", "display_name": strings.Repeat("a", 65)},
	} {
		t.Run(name, func(t *testing.T) {
			app.Post(t, "/api/registrations", token, body).AssertProblem(t, http.StatusBadRequest, "about:blank")
		})
	}
}

func TestRedeemingWithoutATokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	app.Post(t, "/api/registrations", "", map[string]string{"invitation_code": app.SeedInvitation(t), "display_name": "Ana"}).
		AssertProblem(t, http.StatusUnauthorized, "about:blank")
}

func TestARegisteredUserCannotRedeemASecondInvitation(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "subject-1")

	var before me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &before)

	second := app.SeedInvitation(t)
	app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": second, "display_name": "Ana"}).
		AssertProblem(t, http.StatusConflict, "/problems/already-registered")

	var after me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &after)

	if after != before {
		t.Errorf("GET /me after the refused redemption = %+v, want %+v", after, before)
	}

	// The refused redemption left the second code unused.
	app.Post(t, "/api/registrations", app.TokenFor("https://issuer.example", "subject-2"),
		map[string]string{"invitation_code": second, "display_name": "Ana"}).AssertJSON(t, http.StatusCreated, &me{})
}

func TestConcurrentRedemptionsOfOneCodeAdmitOneIdentity(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	code := app.SeedInvitation(t)

	const callers = 8

	statuses := make([]int, callers)
	tokens := make([]string, callers)

	for i := range callers {
		tokens[i] = app.TokenFor("https://issuer.example", fmt.Sprintf("racer-%d", i))
	}

	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() {
			statuses[i] = app.Post(t, "/api/registrations", tokens[i], map[string]string{"invitation_code": code, "display_name": "Ana"}).Status
		})
	}

	wg.Wait()

	registered := 0

	for i, status := range statuses {
		switch status {
		case http.StatusCreated:
			registered++

			app.Get(t, "/api/me", tokens[i]).AssertJSON(t, http.StatusOK, &me{})
		case http.StatusUnprocessableEntity:
			app.Get(t, "/api/me", tokens[i]).AssertProblem(t, http.StatusForbidden, "/problems/registration-required")
		default:
			t.Errorf("racer %d: status %d, want 201 or 422", i, status)
		}
	}

	if registered != 1 {
		t.Errorf("%d identities registered with one code, want 1 (statuses %v)", registered, statuses)
	}
}

func TestConcurrentRedemptionsOfTwoCodesByOneIdentityRegisterItOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "subject-1")
	codes := []string{app.SeedInvitation(t), app.SeedInvitation(t)}
	statuses := make([]int, len(codes))

	var wg sync.WaitGroup
	for i, code := range codes {
		wg.Go(func() {
			statuses[i] = app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": code, "display_name": "Ana"}).Status
		})
	}

	wg.Wait()

	slices.Sort(statuses)

	if want := []int{http.StatusCreated, http.StatusConflict}; !slices.Equal(statuses, want) {
		t.Errorf("statuses = %v, want %v", statuses, want)
	}
}

func TestUsersSurviveARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "subject-1")

	var before me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &before)

	app.Rebuild(t)

	var after me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &after)

	if after != before {
		t.Errorf("GET /me after rebuild = %+v, want %+v", after, before)
	}
}

func register(app *apitest.App, t *testing.T, token, code string) *apitest.Response {
	t.Helper()

	return app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": code, "display_name": "Bea"})
}

func TestRegisteringWithAHouseholdCodeJoinsTheHousehold(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")

	var account struct {
		AccountID string `json:"account_id"`
	}
	app.Post(t, "/api/households/"+home.HouseholdID+"/accounts", alice, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	}).AssertJSON(t, http.StatusCreated, &account)

	issued := issueInvitation(t, app, alice, home.HouseholdID)
	bea := app.TokenFor("https://issuer.example", "bea")

	var registered me
	register(app, t, bea, issued.Code).AssertJSON(t, http.StatusCreated, &registered)

	if registered.UserID == "" || registered.IsInstanceAdministrator || registered.DisplayName != "Bea" {
		t.Errorf("registered = %+v, want a User named Bea who is not Instance administrator", registered)
	}

	var got me
	app.Get(t, "/api/me", bea).AssertJSON(t, http.StatusOK, &got)

	if got != registered {
		t.Errorf("GET /me = %+v, want %+v", got, registered)
	}

	var list householdList
	app.Get(t, "/api/households", bea).AssertJSON(t, http.StatusOK, &list)

	if len(list.Items) != 1 || list.Items[0] != home {
		t.Errorf("bea's Households = %+v, want only %+v", list.Items, home)
	}

	var accounts struct {
		Items []struct {
			AccountID string `json:"account_id"`
		} `json:"items"`
	}
	app.Get(t, "/api/households/"+home.HouseholdID+"/accounts", bea).AssertJSON(t, http.StatusOK, &accounts)

	if len(accounts.Items) != 1 || accounts.Items[0].AccountID != account.AccountID {
		t.Errorf("bea sees Accounts %+v, want alice's %s", accounts.Items, account.AccountID)
	}

	if invitations := listInvitations(t, app, alice, home.HouseholdID); len(invitations.Items) != 0 {
		t.Errorf("open Invitations after registering: %+v, want none", invitations.Items)
	}

	// An identity that is already a User still can't register again.
	again := issueInvitation(t, app, alice, home.HouseholdID)
	register(app, t, bea, again.Code).AssertProblem(t, http.StatusConflict, "/problems/already-registered")
}

func TestAnExpiredOrRevokedHouseholdCodeRegistersNobody(t *testing.T) {
	t.Parallel()

	// Registering redeems a seeded Invitation that expires a day after the
	// wall clock, so the test's clock starts there.
	clock := newSettableClock(time.Now().UTC().Truncate(time.Second))
	app := apitest.Start(t, apitest.WithClock(clock.Now))
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")

	expired := issueInvitation(t, app, alice, home.HouseholdID)

	clock.add(24 * time.Hour)

	revoked := issueInvitation(t, app, alice, home.HouseholdID)
	revokeInvitation(t, app, alice, home.HouseholdID, revoked.InvitationID)

	var first apitest.ProblemBody

	for i, tc := range []struct{ name, code string }{
		{"unknown", "not-a-code"},
		{"expired", expired.Code},
		{"revoked", revoked.Code},
	} {
		token := app.TokenFor("https://issuer.example", "caller-with-"+tc.name+"-code")

		p := register(app, t, token, tc.code).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-invitation")
		if i == 0 {
			first = p
		} else if p != first {
			t.Errorf("%s: problem %+v differs from %+v", tc.name, p, first)
		}

		app.Get(t, "/api/me", token).AssertProblem(t, http.StatusForbidden, "/problems/registration-required")
	}

	if members := listMembers(t, app, alice, home.HouseholdID); len(members) != 1 {
		t.Errorf("members = %+v, want only alice", members)
	}
}

func TestConcurrentRegistrationsWithOneHouseholdCodeAdmitOneMember(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")
	issued := issueInvitation(t, app, alice, home.HouseholdID)

	const callers = 4

	tokens := make([]string, callers)
	for i := range tokens {
		tokens[i] = app.TokenFor("https://issuer.example", fmt.Sprintf("racer-%d", i))
	}

	statuses := make([]int, callers)

	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Go(func() { statuses[i] = register(app, t, token, issued.Code).Status })
	}

	wg.Wait()

	registered := 0

	for i, status := range statuses {
		switch status {
		case http.StatusCreated:
			registered++

			var list householdList
			app.Get(t, "/api/households", tokens[i]).AssertJSON(t, http.StatusOK, &list)

			if len(list.Items) != 1 || list.Items[0] != home {
				t.Errorf("racer %d's Households = %+v, want only %+v", i, list.Items, home)
			}
		case http.StatusUnprocessableEntity:
			app.Get(t, "/api/me", tokens[i]).AssertProblem(t, http.StatusForbidden, "/problems/registration-required")
		default:
			t.Errorf("racer %d: status %d, want 201 or 422", i, status)
		}
	}

	if registered != 1 {
		t.Errorf("%d identities registered with one code, want 1 (statuses %v)", registered, statuses)
	}

	if members := listMembers(t, app, alice, home.HouseholdID); len(members) != 2 {
		t.Errorf("members = %+v, want alice and one racer", members)
	}
}
