//go:build integration

package http_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

func join(app *apitest.App, t *testing.T, token, code string) *apitest.Response {
	t.Helper()

	return app.Post(t, "/api/memberships", token, map[string]string{"invitation_code": code})
}

func TestAUserWithAHouseholdJoinsASecondOne(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")
	home := createHousehold(t, app, alice, "Home")
	bobs := createHousehold(t, app, bob, "Bob's")

	var account struct {
		AccountID string `json:"account_id"`
	}
	app.Post(t, "/api/households/"+home.HouseholdID+"/accounts", alice, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	}).AssertJSON(t, http.StatusCreated, &account)

	issued := issueInvitation(t, app, alice, home.HouseholdID)

	var joined household
	join(app, t, bob, issued.Code).AssertJSON(t, http.StatusCreated, &joined)

	if joined != home {
		t.Errorf("joined = %+v, want %+v", joined, home)
	}

	var list householdList
	app.Get(t, "/api/households", bob).AssertJSON(t, http.StatusOK, &list)

	if len(list.Items) != 2 || list.Items[0] != home || list.Items[1] != bobs {
		t.Errorf("bob's Households = %+v, want %+v then %+v", list.Items, home, bobs)
	}

	var accounts struct {
		Items []struct {
			AccountID string `json:"account_id"`
		} `json:"items"`
	}
	app.Get(t, "/api/households/"+home.HouseholdID+"/accounts", bob).AssertJSON(t, http.StatusOK, &accounts)

	if len(accounts.Items) != 1 || accounts.Items[0].AccountID != account.AccountID {
		t.Errorf("bob sees Accounts %+v, want alice's %s", accounts.Items, account.AccountID)
	}

	if invitations := listInvitations(t, app, bob, home.HouseholdID); len(invitations.Items) != 0 {
		t.Errorf("open Invitations after joining: %+v, want none", invitations.Items)
	}
}

func TestJoiningAHouseholdYouBelongToLeavesTheCodeUnused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")
	home := createHousehold(t, app, alice, "Home")

	issued := issueInvitation(t, app, alice, home.HouseholdID)

	join(app, t, alice, issued.Code).AssertProblem(t, http.StatusConflict, "/problems/already-a-member")

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 1 {
		t.Fatalf("open Invitations after the refusal: %+v, want %s still open", list.Items, issued.InvitationID)
	}

	var joined household
	join(app, t, bob, issued.Code).AssertJSON(t, http.StatusCreated, &joined)

	if joined != home {
		t.Errorf("bob joined %+v, want %+v", joined, home)
	}
}

func TestEveryInvalidCodeGetsTheSameProblem(t *testing.T) {
	t.Parallel()

	// Registering redeems a seeded Invitation that expires a day after the
	// wall clock, so the test's clock starts there.
	start := time.Now().UTC().Truncate(time.Second)
	clock := newSettableClock(start)
	app := apitest.Start(t, apitest.WithClock(clock.Now))
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")
	carol := app.Register(t, "https://issuer.example", "carol")
	home := createHousehold(t, app, alice, "Home")

	bootstrap, err := app.RunBootstrap(t)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	redeemed := issueInvitation(t, app, alice, home.HouseholdID)
	join(app, t, bob, redeemed.Code).AssertJSON(t, http.StatusCreated, &household{})

	expired := issueInvitation(t, app, alice, home.HouseholdID)

	clock.add(24 * time.Hour)

	revoked := issueInvitation(t, app, alice, home.HouseholdID)
	revokeInvitation(t, app, alice, home.HouseholdID, revoked.InvitationID)

	var first apitest.ProblemBody

	for i, tc := range []struct{ name, code string }{
		{"unknown", "not-a-code"},
		{"expired", expired.Code},
		{"redeemed", redeemed.Code},
		{"revoked", revoked.Code},
		{"bootstrap", bootstrap},
	} {
		p := join(app, t, carol, tc.code).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-invitation")
		if i == 0 {
			first = p
		} else if p != first {
			t.Errorf("%s: problem %+v differs from %+v", tc.name, p, first)
		}
	}

	var list householdList
	app.Get(t, "/api/households", carol).AssertJSON(t, http.StatusOK, &list)

	if len(list.Items) != 0 {
		t.Errorf("carol's Households = %+v, want none", list.Items)
	}
}

// A member of the code's own Household gets the same problem for a code
// that can't be redeemed, so already-a-member never tells them it exists.
func TestAMemberWithAnInvalidCodeForTheirHouseholdGetsTheSameProblem(t *testing.T) {
	t.Parallel()

	start := time.Now().UTC().Truncate(time.Second)
	clock := newSettableClock(start)
	app := apitest.Start(t, apitest.WithClock(clock.Now))
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")
	home := createHousehold(t, app, alice, "Home")

	redeemed := issueInvitation(t, app, alice, home.HouseholdID)
	join(app, t, bob, redeemed.Code).AssertJSON(t, http.StatusCreated, &household{})

	expired := issueInvitation(t, app, alice, home.HouseholdID)

	clock.add(24 * time.Hour)

	revoked := issueInvitation(t, app, alice, home.HouseholdID)
	revokeInvitation(t, app, alice, home.HouseholdID, revoked.InvitationID)

	unknown := join(app, t, alice, "not-a-code").
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-invitation")

	for _, tc := range []struct{ name, token, code string }{
		{"revoked", alice, revoked.Code},
		{"expired", alice, expired.Code},
		{"redeemed by another", alice, redeemed.Code},
		{"redeemed by themselves", bob, redeemed.Code},
	} {
		p := join(app, t, tc.token, tc.code).
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-invitation")
		if p != unknown {
			t.Errorf("%s: problem %+v differs from %+v", tc.name, p, unknown)
		}
	}
}

func TestConcurrentRedemptionsOfOneCodeSucceedOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")

	tokens := make([]string, 4)
	for i := range tokens {
		tokens[i] = app.Register(t, "https://issuer.example", fmt.Sprintf("user-%d", i))
	}

	issued := issueInvitation(t, app, alice, home.HouseholdID)

	statuses := make([]int, len(tokens))

	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Go(func() { statuses[i] = join(app, t, token, issued.Code).Status })
	}

	wg.Wait()

	created := 0

	for i, status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusUnprocessableEntity:
		default:
			t.Errorf("redemption %d: status %d, want 201 or 422", i, status)
		}
	}

	if created != 1 {
		t.Errorf("statuses = %v, want exactly one 201", statuses)
	}

	joined := 0

	for _, token := range tokens {
		var list householdList
		app.Get(t, "/api/households", token).AssertJSON(t, http.StatusOK, &list)
		joined += len(list.Items)
	}

	if joined != 1 {
		t.Errorf("%d Users joined, want exactly one", joined)
	}
}

func TestAnUnregisteredCallerMustRegisterToJoin(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")
	issued := issueInvitation(t, app, alice, home.HouseholdID)

	stranger := app.TokenFor("https://issuer.example", "stranger")
	join(app, t, stranger, issued.Code).AssertProblem(t, http.StatusForbidden, "/problems/registration-required")

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 1 {
		t.Errorf("open Invitations after the refusal: %+v, want %s still open", list.Items, issued.InvitationID)
	}
}
