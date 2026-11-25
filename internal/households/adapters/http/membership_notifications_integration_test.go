//go:build integration

package http_test

import (
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// The Household is told when someone joins, leaves or is removed, through
// deferred handlers (join-household spec, "Households: Notifications",
// stories 20-21).

type memberRef struct {
	UserID      string  `json:"user_id"`
	DisplayName *string `json:"display_name"`
}

// notification is a Notification as the API returns it. The pointer fields
// are nil when the response leaves them out.
type notification struct {
	NotificationID    string     `json:"notification_id"`
	Kind              string     `json:"kind"`
	StatementImportID *string    `json:"statement_import_id"`
	ConnectionID      *string    `json:"connection_id"`
	AccountID         *string    `json:"account_id"`
	SuspectedDupes    *int       `json:"suspected_duplicates"`
	Member            *memberRef `json:"member"`
	RemovedBy         *memberRef `json:"removed_by"`
	CreatedAt         string     `json:"created_at"`
	Read              bool       `json:"read"`
}

func notificationsPath(household string) string {
	return "/api/households/" + household + "/notifications"
}

func notificationsOf(t *testing.T, app *apitest.App, token, household string) []notification {
	t.Helper()

	var list struct {
		Items []notification `json:"items"`
	}
	app.Get(t, notificationsPath(household), token).AssertJSON(t, http.StatusOK, &list)

	return list.Items
}

func ref(userID, name string) *memberRef { return &memberRef{UserID: userID, DisplayName: named(name)} }

func sameRef(got, want *memberRef) bool {
	if got == nil || want == nil {
		return got == want
	}

	return got.UserID == want.UserID && (got.DisplayName == nil) == (want.DisplayName == nil) &&
		(got.DisplayName == nil || *got.DisplayName == *want.DisplayName)
}

// assertMembershipNotification checks n is an unread Notification of kind
// about member, removed by removedBy (nil unless removed), with none of the
// Statement import fields.
func assertMembershipNotification(t *testing.T, n notification, kind string, member, removedBy *memberRef) {
	t.Helper()

	if n.NotificationID == "" || n.Kind != kind || n.CreatedAt == "" || n.Read ||
		!sameRef(n.Member, member) || !sameRef(n.RemovedBy, removedBy) {
		t.Errorf("notification = %+v (member %+v, removed by %+v), want an unread %s about %+v removed by %+v",
			n, n.Member, n.RemovedBy, kind, member, removedBy)
	}

	if n.StatementImportID != nil || n.ConnectionID != nil || n.AccountID != nil || n.SuspectedDupes != nil {
		t.Errorf("notification = %+v, want no Statement import fields", n)
	}
}

func TestCreatingAHouseholdRaisesNoNotification(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, _ := registerAs(t, app, "alice", "Alice")
	home := createHousehold(t, app, alice, "Home")

	app.Drain(t)

	if got := notificationsOf(t, app, alice, home.HouseholdID); len(got) != 0 {
		t.Errorf("notifications = %+v, want none", got)
	}
}

func TestJoiningAHouseholdNotifiesItsMembers(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, _ := registerAs(t, app, "alice", "Alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	home := createHousehold(t, app, alice, "Home")
	joinedBy(t, app, alice, bob, home.HouseholdID)

	app.Drain(t)

	for _, token := range []string{alice, bob} {
		got := notificationsOf(t, app, token, home.HouseholdID)
		if len(got) != 1 {
			t.Fatalf("notifications = %+v, want one about Bob joining", got)
		}

		assertMembershipNotification(t, got[0], "MEMBER_JOINED", ref(bobID, "Bob"), nil)
	}
}

func TestRegisteringWithAHouseholdCodeNotifiesItsMembers(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, _ := registerAs(t, app, "alice", "Alice")
	home := createHousehold(t, app, alice, "Home")
	bea := app.TokenFor("https://issuer.example", "bea")

	var registered me
	register(app, t, bea, issueInvitation(t, app, alice, home.HouseholdID).Code).
		AssertJSON(t, http.StatusCreated, &registered)

	app.Drain(t)

	got := notificationsOf(t, app, alice, home.HouseholdID)
	if len(got) != 1 {
		t.Fatalf("notifications = %+v, want one about Bea joining", got)
	}

	assertMembershipNotification(t, got[0], "MEMBER_JOINED", ref(registered.UserID, "Bea"), nil)
}

// A member who left is no longer in the members list, but the
// Notification still names them.
func TestLeavingNotifiesTheRemainingMembers(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, _ := registerAs(t, app, "alice", "Alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	home := createHousehold(t, app, alice, "Home")
	joinedBy(t, app, alice, bob, home.HouseholdID)
	assertRemoved(t, removeMember(app, t, bob, home.HouseholdID, bobID))

	app.Drain(t)

	got := notificationsOf(t, app, alice, home.HouseholdID)
	if len(got) != 2 {
		t.Fatalf("notifications = %+v, want Bob leaving, then Bob joining", got)
	}

	assertMembershipNotification(t, got[0], "MEMBER_LEFT", ref(bobID, "Bob"), nil)
	assertMembershipNotification(t, got[1], "MEMBER_JOINED", ref(bobID, "Bob"), nil)
}

func TestRemovingAMemberNotifiesTheRemainingMembersOfWhoRemovedThem(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, aliceID := registerAs(t, app, "alice", "Alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	carol, _ := registerAs(t, app, "carol", "Carol")
	home := createHousehold(t, app, alice, "Home")
	joinedBy(t, app, alice, bob, home.HouseholdID)
	joinedBy(t, app, alice, carol, home.HouseholdID)
	assertRemoved(t, removeMember(app, t, alice, home.HouseholdID, bobID))

	app.Drain(t)

	got := notificationsOf(t, app, carol, home.HouseholdID)
	if len(got) != 3 {
		t.Fatalf("notifications = %+v, want Bob removed, Carol joined, Bob joined", got)
	}

	assertMembershipNotification(t, got[0], "MEMBER_REMOVED", ref(bobID, "Bob"), ref(aliceID, "Alice"))

	// The names are read when listing, so a rename shows; the fold survives
	// a rebuild.
	app.Patch(t, "/api/me", bob, map[string]string{"display_name": "Robert"}).AssertJSON(t, http.StatusOK, &me{})
	app.Rebuild(t)

	got = notificationsOf(t, app, carol, home.HouseholdID)
	if len(got) != 3 {
		t.Fatalf("notifications after a rebuild = %+v, want the same three", got)
	}

	assertMembershipNotification(t, got[0], "MEMBER_REMOVED", ref(bobID, "Robert"), ref(aliceID, "Alice"))
}

// Read state stays per User, so a new member sees the Notifications raised
// before they joined as unread (ADR-0003: they see everything).
func TestANewMemberSeesEarlierNotificationsUnread(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice, _ := registerAs(t, app, "alice", "Alice")
	bob, bobID := registerAs(t, app, "bob", "Bob")
	carol, carolID := registerAs(t, app, "carol", "Carol")
	home := createHousehold(t, app, alice, "Home")
	joinedBy(t, app, alice, bob, home.HouseholdID)
	app.Drain(t)

	before := notificationsOf(t, app, alice, home.HouseholdID)
	if len(before) != 1 {
		t.Fatalf("notifications = %+v, want one about Bob joining", before)
	}

	bobJoined := before[0]

	var read notification
	app.PostWithKey(t, notificationsPath(home.HouseholdID)+"/"+bobJoined.NotificationID+"/read", alice, "read-1", nil).
		AssertJSON(t, http.StatusOK, &read)

	if read.NotificationID != bobJoined.NotificationID || !read.Read || !sameRef(read.Member, ref(bobID, "Bob")) {
		t.Errorf("marked read = %+v (member %+v), want Bob's joining, read", read, read.Member)
	}

	joinedBy(t, app, alice, carol, home.HouseholdID)
	app.Drain(t)

	got := notificationsOf(t, app, carol, home.HouseholdID)
	if len(got) != 2 {
		t.Fatalf("carol's notifications = %+v, want Carol joined, Bob joined", got)
	}

	assertMembershipNotification(t, got[0], "MEMBER_JOINED", ref(carolID, "Carol"), nil)
	assertMembershipNotification(t, got[1], "MEMBER_JOINED", ref(bobID, "Bob"), nil)

	if got := notificationsOf(t, app, alice, home.HouseholdID); len(got) != 2 || !got[1].Read || got[0].Read {
		t.Errorf("alice's notifications = %+v, want Carol's joining unread and Bob's read", got)
	}
}
