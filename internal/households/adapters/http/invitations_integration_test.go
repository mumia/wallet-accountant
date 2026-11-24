//go:build integration

package http_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type issuedInvitation struct {
	InvitationID string    `json:"invitation_id"`
	Code         string    `json:"code"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type userRef struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
}

type invitation struct {
	InvitationID string    `json:"invitation_id"`
	IssuedBy     userRef   `json:"issued_by"`
	IssuedAt     time.Time `json:"issued_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type invitationList struct {
	Items []invitation `json:"items"`
}

// settableClock is a clock a test moves instead of sleeping.
type settableClock struct{ now atomic.Pointer[time.Time] }

func newSettableClock(start time.Time) *settableClock {
	c := &settableClock{}
	c.set(start)

	return c
}

func (c *settableClock) set(t time.Time)     { c.now.Store(&t) }
func (c *settableClock) Now() time.Time      { return *c.now.Load() }
func (c *settableClock) add(d time.Duration) { c.set(c.Now().Add(d)) }

func invitationsPath(household string) string {
	return "/api/households/" + household + "/invitations"
}

func issueInvitation(t *testing.T, app *apitest.App, token, household string) issuedInvitation {
	t.Helper()

	var issued issuedInvitation
	app.Do(t, http.MethodPost, invitationsPath(household), token, nil).AssertJSON(t, http.StatusCreated, &issued)

	return issued
}

func listInvitations(t *testing.T, app *apitest.App, token, household string) invitationList {
	t.Helper()

	var list invitationList
	app.Get(t, invitationsPath(household), token).AssertJSON(t, http.StatusOK, &list)

	return list
}

func TestIssuingAnInvitationThenListingIt(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)
	clock := newSettableClock(start)
	app := apitest.Start(t, apitest.WithClock(clock.Now))
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")

	var me struct {
		UserID string `json:"user_id"`
	}
	app.Get(t, "/api/me", alice).AssertJSON(t, http.StatusOK, &me)

	first := issueInvitation(t, app, alice, home.HouseholdID)

	if first.InvitationID == "" || first.Code == "" {
		t.Fatalf("issued = %+v, want an ID and a code", first)
	}

	if want := start.Add(24 * time.Hour); !first.ExpiresAt.Equal(want) {
		t.Errorf("expires_at = %s, want %s", first.ExpiresAt, want)
	}

	clock.add(time.Minute)
	second := issueInvitation(t, app, alice, home.HouseholdID)

	if second.InvitationID == first.InvitationID || second.Code == first.Code {
		t.Fatalf("second issue %+v reused the first %+v: each issue is a new Invitation", second, first)
	}

	res := app.Get(t, invitationsPath(home.HouseholdID), alice)

	if strings.Contains(string(res.Body), first.Code) || strings.Contains(string(res.Body), second.Code) ||
		strings.Contains(string(res.Body), `"code"`) {
		t.Errorf("the list shows a code: %s", res.Body)
	}

	var list invitationList
	res.AssertJSON(t, http.StatusOK, &list)

	want := []invitation{
		{
			InvitationID: first.InvitationID,
			IssuedBy:     userRef{UserID: me.UserID, DisplayName: "Ana"},
			IssuedAt:     start,
			ExpiresAt:    start.Add(24 * time.Hour),
		},
		{
			InvitationID: second.InvitationID,
			IssuedBy:     userRef{UserID: me.UserID, DisplayName: "Ana"},
			IssuedAt:     start.Add(time.Minute),
			ExpiresAt:    start.Add(time.Minute + 24*time.Hour),
		},
	}

	if len(list.Items) != len(want) {
		t.Fatalf("list = %+v, want %+v", list.Items, want)
	}

	for i := range want {
		got := list.Items[i]
		if got.InvitationID != want[i].InvitationID || got.IssuedBy != want[i].IssuedBy ||
			!got.IssuedAt.Equal(want[i].IssuedAt) || !got.ExpiresAt.Equal(want[i].ExpiresAt) {
			t.Errorf("item %d = %+v, want %+v", i, got, want[i])
		}
	}
}

func TestAnExpiredInvitationDropsOutOfTheList(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)
	clock := newSettableClock(start)
	app := apitest.Start(t, apitest.WithClock(clock.Now))
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")

	issued := issueInvitation(t, app, alice, home.HouseholdID)

	clock.set(start.Add(24*time.Hour - time.Second))

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 1 || list.Items[0].InvitationID != issued.InvitationID {
		t.Fatalf("a second before expiry: %+v, want %s", list.Items, issued.InvitationID)
	}

	clock.set(start.Add(24 * time.Hour))

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 0 {
		t.Errorf("at expiry: %+v, want none", list.Items)
	}
}

func TestInvitationsOfAnotherHouseholdAreNotFound(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")
	home := createHousehold(t, app, alice, "Home")
	createHousehold(t, app, bob, "Bob's")

	issueInvitation(t, app, alice, home.HouseholdID)

	app.Do(t, http.MethodPost, invitationsPath(home.HouseholdID), bob, nil).
		AssertProblem(t, http.StatusNotFound, "about:blank")
	app.Get(t, invitationsPath(home.HouseholdID), bob).
		AssertProblem(t, http.StatusNotFound, "about:blank")

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 1 {
		t.Errorf("after another Household's member tried: %+v, want only alice's", list.Items)
	}
}

func TestReplayingCreateInvitationReturnsTheSameCode(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")
	key := http.Header{"Idempotency-Key": {"invite-1"}}

	var first, replay issuedInvitation
	app.DoWithHeader(t, http.MethodPost, invitationsPath(home.HouseholdID), alice, key, nil).
		AssertJSON(t, http.StatusCreated, &first)
	app.DoWithHeader(t, http.MethodPost, invitationsPath(home.HouseholdID), alice, key, nil).
		AssertJSON(t, http.StatusCreated, &replay)

	if replay != first {
		t.Errorf("replay = %+v, want the stored %+v", replay, first)
	}

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 1 {
		t.Errorf("Invitations after a replay: %+v, want only %s", list.Items, first.InvitationID)
	}
}

func TestInvitationsSurviveARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")

	issueInvitation(t, app, alice, home.HouseholdID)
	before := listInvitations(t, app, alice, home.HouseholdID)

	app.Rebuild(t)

	after := listInvitations(t, app, alice, home.HouseholdID)
	if len(after.Items) != 1 || after.Items[0] != before.Items[0] {
		t.Errorf("after rebuild = %+v, want %+v", after.Items, before.Items)
	}
}

func invitationPath(household, invitation string) string {
	return invitationsPath(household) + "/" + invitation
}

func revokeInvitation(t *testing.T, app *apitest.App, token, household, invitation string) {
	t.Helper()

	res := app.Do(t, http.MethodDelete, invitationPath(household, invitation), token, nil)
	if res.Status != http.StatusNoContent {
		t.Fatalf("revoke %s: status %d, want 204; body: %s", invitation, res.Status, res.Body)
	}
}

func TestRevokingAnInvitationDropsItFromTheList(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")

	revoked := issueInvitation(t, app, alice, home.HouseholdID)
	kept := issueInvitation(t, app, alice, home.HouseholdID)

	revokeInvitation(t, app, alice, home.HouseholdID, revoked.InvitationID)

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 1 || list.Items[0].InvitationID != kept.InvitationID {
		t.Errorf("after revoking %s: %+v, want only %s", revoked.InvitationID, list.Items, kept.InvitationID)
	}

	app.Do(t, http.MethodDelete, invitationPath(home.HouseholdID, revoked.InvitationID), alice, nil).
		AssertProblem(t, http.StatusNotFound, "about:blank")

	app.Rebuild(t)

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 1 || list.Items[0].InvitationID != kept.InvitationID {
		t.Errorf("after a rebuild: %+v, want only %s", list.Items, kept.InvitationID)
	}
}

func TestAMemberCanRevokeAnotherMembersInvitation(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	home := createHousehold(t, app, alice, "Home")
	app.SeedMember(t, home.HouseholdID, bobID, time.Now())

	issued := issueInvitation(t, app, alice, home.HouseholdID)

	revokeInvitation(t, app, bob, home.HouseholdID, issued.InvitationID)

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 0 {
		t.Errorf("after bob revoked alice's Invitation: %+v, want none", list.Items)
	}
}

func TestRevokingAnotherHouseholdsInvitationIsNotFound(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")
	home := createHousehold(t, app, alice, "Home")
	bobs := createHousehold(t, app, bob, "Bob's")

	issued := issueInvitation(t, app, alice, home.HouseholdID)

	// bob isn't a member of alice's Household.
	app.Do(t, http.MethodDelete, invitationPath(home.HouseholdID, issued.InvitationID), bob, nil).
		AssertProblem(t, http.StatusNotFound, "about:blank")
	// bob is a member of his own, which alice's Invitation isn't of.
	app.Do(t, http.MethodDelete, invitationPath(bobs.HouseholdID, issued.InvitationID), bob, nil).
		AssertProblem(t, http.StatusNotFound, "about:blank")

	if list := listInvitations(t, app, alice, home.HouseholdID); len(list.Items) != 1 {
		t.Errorf("after bob tried to revoke it: %+v, want alice's Invitation still open", list.Items)
	}
}

func TestRevokingAnInvitationThatIsNotOpenIsNotFound(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)
	clock := newSettableClock(start)
	app := apitest.Start(t, apitest.WithClock(clock.Now))
	alice := app.Register(t, "https://issuer.example", "alice")
	home := createHousehold(t, app, alice, "Home")

	issued := issueInvitation(t, app, alice, home.HouseholdID)

	app.Do(t, http.MethodDelete, invitationPath(home.HouseholdID, "019a8b2c-0000-7000-8000-0000000000ff"), alice, nil).
		AssertProblem(t, http.StatusNotFound, "about:blank")

	// The contract counts an expired Invitation as not open.
	clock.set(start.Add(24 * time.Hour))
	app.Do(t, http.MethodDelete, invitationPath(home.HouseholdID, issued.InvitationID), alice, nil).
		AssertProblem(t, http.StatusNotFound, "about:blank")
}
