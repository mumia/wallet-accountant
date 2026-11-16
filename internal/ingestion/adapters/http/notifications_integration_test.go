//go:build integration

package http_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// notification is a Household's Notification as the API returns it.
type notification struct {
	NotificationID      string `json:"notification_id"`
	Kind                string `json:"kind"`
	StatementImportID   string `json:"statement_import_id"`
	ConnectionID        string `json:"connection_id"`
	AccountID           string `json:"account_id"`
	SuspectedDuplicates int    `json:"suspected_duplicates"`
	CreatedAt           string `json:"created_at"`
	Read                bool   `json:"read"`
}

func (f fixture) notificationsPath() string {
	return "/households/" + f.household + "/notifications"
}

func (f fixture) notifications(t *testing.T) []notification {
	t.Helper()

	var list struct {
		Items []notification `json:"items"`
	}

	f.app.Get(t, f.notificationsPath(), f.token).AssertJSON(t, http.StatusOK, &list)

	return list.Items
}

func (f fixture) readPath(id string) string {
	return f.notificationsPath() + "/" + id + "/read"
}

// notified uploads overlapWithoutIDs, which brings two Suspected
// duplicates, drains the deferred handlers and returns the import with the
// one Notification it raised.
func notified(t *testing.T, f suspectedFixture) (statementImport, notification) {
	t.Helper()

	var si statementImport
	f.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	f.app.Drain(t)

	got := f.notifications(t)
	if len(got) != 1 {
		t.Fatalf("notifications = %+v, want the one about the Suspected duplicates", got)
	}

	return si, got[0]
}

// A Statement import with Suspected duplicates notifies the Household,
// through a deferred handler, with a link to the import (spec stories 40a,
// 40f; ADR-0013).
func TestSuspectedDuplicatesNotifyTheHousehold(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newSuspectedFixture(t, app, "alice")

	si, n := notified(t, f)

	if n.NotificationID == "" || n.Kind != "SUSPECTED_DUPLICATES_FOUND" || n.StatementImportID != si.StatementImportID ||
		n.ConnectionID != f.connection || n.AccountID != f.account || n.SuspectedDuplicates != 2 ||
		n.CreatedAt == "" || n.Read {
		t.Errorf("notification = %+v, want an unread one about import %s and its 2 Suspected duplicates",
			n, si.StatementImportID)
	}

	// The handler continues the trace of the upload that caused it.
	var uploadTrace, handlerTrace string

	for _, s := range app.Spans.GetSpans() {
		switch {
		case s.Name == "POST "+f.imports():
			uploadTrace = s.SpanContext.TraceID().String()
		case strings.HasSuffix(s.Name, "statementimport.recorded.v1") && strings.HasPrefix(s.Name, "handle "):
			handlerTrace = s.SpanContext.TraceID().String()
		}
	}

	if uploadTrace == "" || handlerTrace != uploadTrace {
		t.Errorf("handler trace = %q, upload trace = %q, want the handler in the upload's trace", handlerTrace, uploadTrace)
	}

	// The fold is rebuilt from its events.
	app.Rebuild(t)

	if got := f.notifications(t); len(got) != 1 || got[0] != n {
		t.Errorf("after rebuild notifications = %+v, want %+v", got, n)
	}
}

func TestAnImportWithoutSuspectedDuplicatesNotifiesNobody(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	f.confirm(t, statement, "2500.00")
	f.upload(t, "", otherStatement, "2454.90", "").AssertJSON(t, http.StatusCreated, &statementImport{})

	f.app.Drain(t)

	if got := f.notifications(t); len(got) != 0 {
		t.Errorf("notifications = %+v, want none", got)
	}
}

func TestMarkingANotificationRead(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	_, n := notified(t, f)

	var read notification
	f.app.PostWithKey(t, f.readPath(n.NotificationID), f.token, "read-1", nil).AssertJSON(t, http.StatusOK, &read)

	want := n
	want.Read = true

	if read != want {
		t.Errorf("read = %+v, want %+v", read, want)
	}

	if got := f.notifications(t); len(got) != 1 || got[0] != want {
		t.Errorf("notifications = %+v, want %+v", got, want)
	}

	// Marking it read again changes nothing, with or without the key.
	f.app.PostWithKey(t, f.readPath(n.NotificationID), f.token, "read-1", nil).AssertJSON(t, http.StatusOK, &read)
	f.app.Post(t, f.readPath(n.NotificationID), f.token, nil).AssertJSON(t, http.StatusOK, &read)

	if read != want {
		t.Errorf("read again = %+v, want %+v", read, want)
	}
}

func TestAnotherHouseholdsNotificationsAreNotFound(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := newSuspectedFixture(t, app, "alice")
	bob := newFixture(t, app, "bob")

	_, n := notified(t, alice)

	if got := bob.notifications(t); len(got) != 0 {
		t.Errorf("bob's notifications = %+v, want none", got)
	}

	app.Get(t, alice.notificationsPath(), bob.token).AssertProblem(t, http.StatusNotFound, "about:blank")
	app.Post(t, alice.readPath(n.NotificationID), bob.token, nil).AssertProblem(t, http.StatusNotFound, "about:blank")
	app.Post(t, bob.readPath(n.NotificationID), bob.token, nil).AssertProblem(t, http.StatusNotFound, "about:blank")
	app.Post(t, alice.readPath("019a8b2c-0000-7000-8000-000000000999"), alice.token, nil).
		AssertProblem(t, http.StatusNotFound, "about:blank")

	if got := alice.notifications(t); len(got) != 1 || got[0].Read {
		t.Errorf("alice's notifications = %+v, want hers, unread", got)
	}
}
