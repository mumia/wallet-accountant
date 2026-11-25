//go:build integration

package http_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
	householdshttp "github.com/mumia/wallet-accountant/internal/households/adapters/http"
	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
)

func memberPath(household, user string) string {
	return "/api/households/" + household + "/members/" + user
}

func removeMember(app *apitest.App, t *testing.T, token, household, user string) *apitest.Response {
	t.Helper()

	return app.Do(t, http.MethodDelete, memberPath(household, user), token, nil)
}

func assertRemoved(t *testing.T, res *apitest.Response) {
	t.Helper()

	if res.Status != http.StatusNoContent {
		t.Fatalf("remove member: status %d, want 204; body: %s", res.Status, res.Body)
	}
}

// joinedBy makes the User a member of the Household through an Invitation
// the inviter issues.
func joinedBy(t *testing.T, app *apitest.App, inviter, joiner, householdID string) {
	t.Helper()

	join(app, t, joiner, issueInvitation(t, app, inviter, householdID).Code).
		AssertJSON(t, http.StatusCreated, &household{})
}

func householdsOf(t *testing.T, app *apitest.App, token string) []household {
	t.Helper()

	var list householdList
	app.Get(t, "/api/households", token).AssertJSON(t, http.StatusOK, &list)

	return list.Items
}

func TestRemovingAnotherMember(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(func() time.Time { return creationTime }))
	alice, aliceID := registerAs(t, app, "alice", "Alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	home := createHousehold(t, app, alice, "Home")
	joinedBy(t, app, alice, bob, home.HouseholdID)

	assertRemoved(t, removeMember(app, t, alice, home.HouseholdID, bobID))

	assertMembers(t, listMembers(t, app, alice, home.HouseholdID), []member{
		{UserID: aliceID, DisplayName: named("Alice"), JoinedAt: creationTime},
	})

	if got := app.EventActors(t, householdsapp.HouseholdMemberRemovedType); !slices.Equal(got, []string{aliceID}) {
		t.Errorf("memberremoved actors = %v, want [%s]", got, aliceID)
	}

	// Bob loses access with his next request, and stays a User with no
	// Household.
	app.Get(t, "/api/households/"+home.HouseholdID, bob).AssertProblem(t, http.StatusNotFound, "about:blank")
	app.Get(t, "/api/households/"+home.HouseholdID+"/members", bob).
		AssertProblem(t, http.StatusNotFound, "about:blank")

	if got := householdsOf(t, app, bob); len(got) != 0 {
		t.Errorf("bob's households = %v, want none", got)
	}

	app.Get(t, "/api/me", bob).AssertJSON(t, http.StatusOK, &me{})
	createHousehold(t, app, bob, "Bob's")
}

func TestLeavingAHousehold(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(func() time.Time { return creationTime }))
	alice, aliceID := registerAs(t, app, "alice", "Alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	home := createHousehold(t, app, alice, "Home")
	joinedBy(t, app, alice, bob, home.HouseholdID)

	assertRemoved(t, removeMember(app, t, bob, home.HouseholdID, bobID))

	assertMembers(t, listMembers(t, app, alice, home.HouseholdID), []member{
		{UserID: aliceID, DisplayName: named("Alice"), JoinedAt: creationTime},
	})

	if got := app.EventActors(t, householdsapp.HouseholdMemberRemovedType); !slices.Equal(got, []string{bobID}) {
		t.Errorf("memberremoved actors = %v, want [%s]", got, bobID)
	}

	app.Get(t, "/api/households/"+home.HouseholdID, bob).AssertProblem(t, http.StatusNotFound, "about:blank")

	if got := householdsOf(t, app, bob); len(got) != 0 {
		t.Errorf("bob's households = %v, want none", got)
	}
}

func TestTheLastMemberCannotLeave(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(func() time.Time { return creationTime }))
	alice, aliceID := registerAs(t, app, "alice", "Alice")
	home := createHousehold(t, app, alice, "Home")
	events := app.EventCount(t)

	removeMember(app, t, alice, home.HouseholdID, aliceID).
		AssertProblem(t, http.StatusConflict, householdshttp.LastMemberType)

	if got := app.EventCount(t); got != events {
		t.Errorf("events = %d, want %d: a refusal records nothing", got, events)
	}

	assertMembers(t, listMembers(t, app, alice, home.HouseholdID), []member{
		{UserID: aliceID, DisplayName: named("Alice"), JoinedAt: creationTime},
	})
}

func TestTheLastMemberLeftAfterAnotherLeavesCannotLeave(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, aliceID := registerAs(t, app, "alice", "Alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	home := createHousehold(t, app, alice, "Home")
	joinedBy(t, app, alice, bob, home.HouseholdID)

	assertRemoved(t, removeMember(app, t, bob, home.HouseholdID, bobID))

	removeMember(app, t, alice, home.HouseholdID, aliceID).
		AssertProblem(t, http.StatusConflict, householdshttp.LastMemberType)
}

func TestRemovingAUserWhoIsNotAMemberIsNotFound(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, _ := registerAs(t, app, "alice", "Alice")
	_, carolID := registerAs(t, app, "carol", "Carol")
	home := createHousehold(t, app, alice, "Home")

	for name, user := range map[string]string{
		"a User of another Household": carolID,
		"no User at all":              "019a8b2c-0000-7000-8000-0000000000ff",
	} {
		t.Run(name, func(t *testing.T) {
			removeMember(app, t, alice, home.HouseholdID, user).
				AssertProblem(t, http.StatusNotFound, "about:blank")
		})
	}
}

func TestRemovingInAnotherHouseholdIsNotFound(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, _ := registerAs(t, app, "alice", "Alice")
	carol, carolID := registerAs(t, app, "carol", "Carol")
	dave, daveID := registerAs(t, app, "dave", "Dave")
	createHousehold(t, app, alice, "Home")
	theirs := createHousehold(t, app, carol, "Theirs")
	joinedBy(t, app, carol, dave, theirs.HouseholdID)

	for name, user := range map[string]string{"a member": daveID, "the creator": carolID} {
		t.Run(name, func(t *testing.T) {
			removeMember(app, t, alice, theirs.HouseholdID, user).
				AssertProblem(t, http.StatusNotFound, "about:blank")
		})
	}

	if got := listMembers(t, app, carol, theirs.HouseholdID); len(got) != 2 {
		t.Errorf("their members = %s, want both still there", describe(got))
	}
}

func TestAnInvitationFromARemovedMemberStillWorks(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, _ := registerAs(t, app, "alice", "Alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	carol, carolID := registerAs(t, app, "carol", "Carol")
	home := createHousehold(t, app, alice, "Home")
	joinedBy(t, app, alice, bob, home.HouseholdID)
	fromBob := issueInvitation(t, app, bob, home.HouseholdID)

	assertRemoved(t, removeMember(app, t, alice, home.HouseholdID, bobID))

	join(app, t, carol, fromBob.Code).AssertJSON(t, http.StatusCreated, &household{})

	members := listMembers(t, app, alice, home.HouseholdID)
	if !slices.ContainsFunc(members, func(m member) bool { return m.UserID == carolID }) {
		t.Errorf("members = %s, want Carol among them", describe(members))
	}
}

func TestRejoiningAfterRemoval(t *testing.T) {
	t.Parallel()
	clock := newSettableClock(creationTime)
	app := apitest.Start(t, apitest.WithClock(clock.Now))
	alice, aliceID := registerAs(t, app, "alice", "Alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	home := createHousehold(t, app, alice, "Home")
	joinedBy(t, app, alice, bob, home.HouseholdID)

	clock.add(time.Hour)
	assertRemoved(t, removeMember(app, t, alice, home.HouseholdID, bobID))

	rejoined := creationTime.Add(48 * time.Hour)
	clock.set(rejoined)
	joinedBy(t, app, alice, bob, home.HouseholdID)

	want := []member{
		{UserID: aliceID, DisplayName: named("Alice"), JoinedAt: creationTime},
		{UserID: bobID, DisplayName: named("Bob"), JoinedAt: rejoined},
	}
	assertMembers(t, listMembers(t, app, bob, home.HouseholdID), want)

	if got := householdsOf(t, app, bob); len(got) != 1 || got[0].HouseholdID != home.HouseholdID {
		t.Errorf("bob's households = %v, want only Home", got)
	}

	// A rebuild replays the removal and the rejoin to the same members.
	app.Rebuild(t)
	assertMembers(t, listMembers(t, app, alice, home.HouseholdID), want)
}
