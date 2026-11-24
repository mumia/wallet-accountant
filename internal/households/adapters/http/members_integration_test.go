//go:build integration

package http_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// member is a members list item. DisplayName is nil when the item has no
// display_name, as for a legacy User.
type member struct {
	UserID      string    `json:"user_id"`
	DisplayName *string   `json:"display_name"`
	JoinedAt    time.Time `json:"joined_at"`
}

type memberList struct {
	Items []member `json:"items"`
}

// creationTime is when the members tests create their Household.
var creationTime = time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)

// registerAs makes the identity a User with a display name, and returns its
// bearer token and User ID.
func registerAs(t *testing.T, app *apitest.App, subject, displayName string) (token, userID string) {
	t.Helper()

	token = app.TokenFor("https://issuer.example", subject)

	var me struct {
		UserID string `json:"user_id"`
	}
	app.Post(t, "/api/registrations", token,
		map[string]string{"invitation_code": app.SeedInvitation(t), "display_name": displayName}).
		AssertJSON(t, http.StatusCreated, &me)

	return token, me.UserID
}

func listMembers(t *testing.T, app *apitest.App, token, householdID string) []member {
	t.Helper()

	var list memberList
	app.Get(t, "/api/households/"+householdID+"/members", token).AssertJSON(t, http.StatusOK, &list)

	return list.Items
}

func named(name string) *string { return &name }

func assertMembers(t *testing.T, got, want []member) {
	t.Helper()

	eq := func(a, b member) bool {
		return a.UserID == b.UserID && a.JoinedAt.Equal(b.JoinedAt) &&
			(a.DisplayName == nil) == (b.DisplayName == nil) &&
			(a.DisplayName == nil || *a.DisplayName == *b.DisplayName)
	}

	if !slices.EqualFunc(got, want, eq) {
		t.Errorf("members = %s, want %s", describe(got), describe(want))
	}
}

func describe(members []member) string {
	var b strings.Builder

	for _, m := range members {
		name := "<none>"
		if m.DisplayName != nil {
			name = *m.DisplayName
		}

		fmt.Fprintf(&b, "{%s %q %s} ", m.UserID, name, m.JoinedAt.Format(time.RFC3339Nano))
	}

	return b.String()
}

func TestAHouseholdsCreatorIsItsOnlyMember(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(func() time.Time { return creationTime }))
	alice, aliceID := registerAs(t, app, "alice", "Alice")
	home := createHousehold(t, app, alice, "Home")

	assertMembers(t, listMembers(t, app, alice, home.HouseholdID), []member{
		{UserID: aliceID, DisplayName: named("Alice"), JoinedAt: creationTime},
	})
}

func TestMembersAreListedOldestFirstWithTheirNames(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(func() time.Time { return creationTime }))
	// Bob registers first, so his User ID sorts before Alice's: the list
	// must be ordered by when each joined, not by ID.
	bob, bobID := registerAs(t, app, "bob", "Bob")
	alice, aliceID := registerAs(t, app, "alice", "Alice")
	home := createHousehold(t, app, alice, "Home")

	// Joining through an Invitation (ticket 06) isn't there yet, so Bob's
	// membership is seeded.
	bobJoined := creationTime.Add(26 * time.Hour)
	app.SeedMember(t, home.HouseholdID, bobID, bobJoined)

	want := []member{
		{UserID: aliceID, DisplayName: named("Alice"), JoinedAt: creationTime},
		{UserID: bobID, DisplayName: named("Bob"), JoinedAt: bobJoined},
	}
	assertMembers(t, listMembers(t, app, alice, home.HouseholdID), want)
	assertMembers(t, listMembers(t, app, bob, home.HouseholdID), want)
}

func TestTheMembersListShowsTheCurrentDisplayName(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(func() time.Time { return creationTime }))
	alice, aliceID := registerAs(t, app, "alice", "Alice")
	home := createHousehold(t, app, alice, "Home")

	app.Patch(t, "/api/me", alice, map[string]string{"display_name": "Alice Smith"}).
		AssertJSON(t, http.StatusOK, &me{})

	assertMembers(t, listMembers(t, app, alice, home.HouseholdID), []member{
		{UserID: aliceID, DisplayName: named("Alice Smith"), JoinedAt: creationTime},
	})
}

func TestALegacyMemberHasNoDisplayName(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(func() time.Time { return creationTime }))
	// A User who registered before display names existed has none stored.
	alice := app.RegisterWithoutDisplayName(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")

	res := app.Get(t, "/api/households/"+home.HouseholdID+"/members", alice)

	var raw struct {
		Items []map[string]any `json:"items"`
	}
	res.AssertJSON(t, http.StatusOK, &raw)

	if len(raw.Items) != 1 {
		t.Fatalf("members = %s, want one", res.Body)
	}

	if _, ok := raw.Items[0]["display_name"]; ok {
		t.Errorf("legacy member = %s, want no display_name", res.Body)
	}
}

func TestAnotherHouseholdsMembersAre404(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, _ := registerAs(t, app, "alice", "Alice")
	bob, _ := registerAs(t, app, "bob", "Bob")
	alices := createHousehold(t, app, alice, "Alice's")
	createHousehold(t, app, bob, "Bob's")

	app.Get(t, "/api/households/"+alices.HouseholdID+"/members", bob).
		AssertProblem(t, http.StatusNotFound, "about:blank")
	app.Get(t, "/api/households/019a8b2c-0000-7000-8000-00000000ffff/members", bob).
		AssertProblem(t, http.StatusNotFound, "about:blank")
}
