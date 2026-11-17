package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	statementImport = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000a1")
	connection      = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000a2")
	account         = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000a3")
	member          = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000c2")
)

func suspectedDuplicatesFound() domain.NotificationEvent {
	return domain.NotificationRaised{
		Kind:                domain.SuspectedDuplicatesFound,
		StatementImportID:   statementImport,
		ConnectionID:        connection,
		AccountID:           account,
		SuspectedDuplicates: 2,
	}
}

func readBy(user sharedkernel.ID) domain.NotificationEvent {
	return domain.NotificationRead{UserID: user}
}

func TestSuspectedDuplicatesRaiseANotificationLinkingToTheImport(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Notification).
		When(domain.RaiseSuspectedDuplicatesFound{
			StatementImportID: statementImport, ConnectionID: connection, AccountID: account, SuspectedDuplicates: 2,
		}).
		Then(suspectedDuplicatesFound())
}

func TestANotificationIsRaisedOnce(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Notification, suspectedDuplicatesFound()).
		When(domain.RaiseSuspectedDuplicatesFound{
			StatementImportID: statementImport, ConnectionID: connection, AccountID: account, SuspectedDuplicates: 2,
		}).
		ThenError(domain.ErrNotificationAlreadyRaised)
}

func TestANotificationNeedsSuspectedDuplicates(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Notification).
		When(domain.RaiseSuspectedDuplicatesFound{
			StatementImportID: statementImport, ConnectionID: connection, AccountID: account,
		}).
		ThenError(domain.ErrNoSuspectedDuplicates)
}

func TestAMemberMarksANotificationRead(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Notification, suspectedDuplicatesFound()).
		When(domain.MarkNotificationRead{UserID: creator}).
		Then(domain.NotificationRead{UserID: creator})
}

func TestMarkingReadAgainRecordsNothing(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Notification, suspectedDuplicatesFound(), readBy(creator)).
		When(domain.MarkNotificationRead{UserID: creator}).
		Then()
}

func TestOnlyANotificationThatWasRaisedCanBeRead(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Notification).
		When(domain.MarkNotificationRead{UserID: creator}).
		ThenError(domain.ErrNotificationNotRaised)
}

// Read state is per User: one member reading it leaves it unread for the
// others.
func TestReadStateIsPerUser(t *testing.T) {
	t.Parallel()

	s := decidertest.Given(t, domain.Notification, suspectedDuplicatesFound(), readBy(creator)).State()

	if !s.Raised || s.Kind != domain.SuspectedDuplicatesFound || s.StatementImportID != statementImport ||
		s.ConnectionID != connection || s.AccountID != account || s.SuspectedDuplicates != 2 || s.RaisedAt.IsZero() {
		t.Errorf("state = %+v, want the raised Notification linking to the import", s)
	}

	if !s.IsReadBy(creator) {
		t.Error("the member who read it sees it unread")
	}

	if s.IsReadBy(member) {
		t.Error("another member sees it read")
	}

	decidertest.Given(t, domain.Notification, suspectedDuplicatesFound(), readBy(creator)).
		When(domain.MarkNotificationRead{UserID: member}).
		Then(domain.NotificationRead{UserID: member})
}
