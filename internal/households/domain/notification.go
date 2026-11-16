package domain

import (
	"errors"
	"slices"
	"time"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

var (
	// ErrNotificationAlreadyRaised is returned for raising a Notification
	// twice.
	ErrNotificationAlreadyRaised = errors.New("notification already raised")
	// ErrNoSuspectedDuplicates is returned for a "suspected duplicates
	// found" Notification about an import without Suspected duplicates.
	ErrNoSuspectedDuplicates = errors.New("statement import has no suspected duplicates")
	// ErrNotificationNotRaised is returned for marking read a Notification
	// that was never raised.
	ErrNotificationNotRaised = errors.New("notification not raised")
)

// NotificationKind is what a Notification is about.
type NotificationKind string

// SuspectedDuplicatesFound is a Statement import that produced Suspected
// duplicates, which someone in the Household should review (spec story 40a).
const SuspectedDuplicatesFound NotificationKind = "suspected_duplicates_found"

// NotificationState is a Notification's fold: a Household-level message
// every member sees, linking to the Statement import, with read state per
// User.
type NotificationState struct {
	Raised              bool
	Kind                NotificationKind
	StatementImportID   sharedkernel.ID
	ConnectionID        sharedkernel.ID
	AccountID           sharedkernel.ID
	SuspectedDuplicates int
	RaisedAt            time.Time
	ReadBy              []sharedkernel.ID
}

// IsReadBy reports whether the User has marked the Notification read.
func (s NotificationState) IsReadBy(user sharedkernel.ID) bool {
	return slices.Contains(s.ReadBy, user)
}

type (
	// NotificationCommand is a command to a Notification.
	NotificationCommand interface{ isNotificationCommand() }

	// RaiseSuspectedDuplicatesFound tells the Household that a Statement
	// import produced Suspected duplicates.
	RaiseSuspectedDuplicatesFound struct {
		StatementImportID   sharedkernel.ID
		ConnectionID        sharedkernel.ID
		AccountID           sharedkernel.ID
		SuspectedDuplicates int
	}

	// MarkNotificationRead marks the Notification read for one User.
	MarkNotificationRead struct {
		UserID sharedkernel.ID
	}
)

func (RaiseSuspectedDuplicatesFound) isNotificationCommand() {}
func (MarkNotificationRead) isNotificationCommand()          {}

type (
	// NotificationEvent is a Notification's event.
	NotificationEvent interface{ isNotificationEvent() }

	// NotificationRaised is recorded when a Notification is raised.
	NotificationRaised struct {
		Kind                NotificationKind `json:"kind"`
		StatementImportID   sharedkernel.ID  `json:"statementimportid"`
		ConnectionID        sharedkernel.ID  `json:"connectionid"`
		AccountID           sharedkernel.ID  `json:"accountid"`
		SuspectedDuplicates int              `json:"suspectedduplicates"`
	}

	// NotificationRead is recorded when a User marks a Notification read.
	NotificationRead struct {
		UserID sharedkernel.ID `json:"userid"`
	}
)

func (NotificationRaised) isNotificationEvent() {}
func (NotificationRead) isNotificationEvent()   {}

// Notification is the Notification Decider.
var Notification = decider.Decider[NotificationState, NotificationCommand, NotificationEvent]{
	Decide: decideNotification,
	Evolve: evolveNotification,
}

func decideNotification(s NotificationState, c NotificationCommand) ([]NotificationEvent, error) {
	switch c := c.(type) {
	case RaiseSuspectedDuplicatesFound:
		if s.Raised {
			return nil, ErrNotificationAlreadyRaised
		}

		if c.SuspectedDuplicates <= 0 {
			return nil, ErrNoSuspectedDuplicates
		}

		return []NotificationEvent{NotificationRaised{
			Kind:                SuspectedDuplicatesFound,
			StatementImportID:   c.StatementImportID,
			ConnectionID:        c.ConnectionID,
			AccountID:           c.AccountID,
			SuspectedDuplicates: c.SuspectedDuplicates,
		}}, nil
	case MarkNotificationRead:
		if !s.Raised {
			return nil, ErrNotificationNotRaised
		}

		if s.IsReadBy(c.UserID) {
			return nil, nil
		}

		return []NotificationEvent{NotificationRead(c)}, nil
	default:
		return nil, nil
	}
}

func evolveNotification(s NotificationState, e decider.Envelope[NotificationEvent]) NotificationState {
	switch ev := e.Event.(type) {
	case NotificationRaised:
		s.Raised = true
		s.Kind = ev.Kind
		s.StatementImportID, s.ConnectionID, s.AccountID = ev.StatementImportID, ev.ConnectionID, ev.AccountID
		s.SuspectedDuplicates = ev.SuspectedDuplicates
		s.RaisedAt = e.Time
	case NotificationRead:
		if !s.IsReadBy(ev.UserID) {
			s.ReadBy = append(slices.Clone(s.ReadBy), ev.UserID)
		}
	}

	return s
}
