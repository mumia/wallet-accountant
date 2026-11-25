package app_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

var (
	household  = uuid.MustParse("019a8b2c-0000-7000-8000-0000000000b1")
	member     = uuid.MustParse("019a8b2c-0000-7000-8000-0000000000c2")
	other      = uuid.MustParse("019a8b2c-0000-7000-8000-0000000000c3")
	invitation = uuid.MustParse("019a8b2c-0000-7000-8000-0000000000d1")
)

func householdEvent(t *testing.T, eventType string, actor uuid.UUID, data any) eventstore.CloudEvent {
	t.Helper()

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	return eventstore.CloudEvent{
		ID: uuid.New(), Source: app.Source, Type: eventType, Subject: household, HouseholdID: household,
		StreamType: app.HouseholdStreamType, ActorID: actor, Data: raw,
	}
}

// The membership notifier's handler raises a Notification for joining,
// leaving and removal, and nothing for the member that creating a
// Household adds (join-household spec, "Households: Notifications").
func TestMembershipNotificationIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event eventstore.CloudEvent
		want  domain.NotificationCommand
	}{
		{
			name: "a member joined through an Invitation",
			event: householdEvent(t, app.HouseholdMemberAddedType, member,
				domain.HouseholdMemberAdded{UserID: member, InvitationID: invitation}),
			want: domain.RaiseMemberJoined{MemberID: member},
		},
		{
			name:  "the creator of a new Household",
			event: householdEvent(t, app.HouseholdMemberAddedType, member, domain.HouseholdMemberAdded{UserID: member}),
		},
		{
			name:  "a member left",
			event: householdEvent(t, app.HouseholdMemberRemovedType, member, domain.HouseholdMemberRemoved{UserID: member}),
			want:  domain.RaiseMemberLeft{MemberID: member},
		},
		{
			name:  "a member was removed",
			event: householdEvent(t, app.HouseholdMemberRemovedType, other, domain.HouseholdMemberRemoved{UserID: member}),
			want:  domain.RaiseMemberRemoved{MemberID: member, RemovedBy: other},
		},
		{
			name: "another Household event",
			event: householdEvent(t, app.HouseholdCreatedType, member,
				domain.HouseholdCreated{Name: "Home", BaseCurrency: "EUR"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok, err := app.MembershipNotificationIn(tt.event)
			if err != nil {
				t.Fatal(err)
			}

			if ok != (tt.want != nil) || got != tt.want {
				t.Errorf("MembershipNotificationIn = %#v (%t), want %#v", got, ok, tt.want)
			}
		})
	}
}

func TestAnUnreadableMembershipEventIsAnError(t *testing.T) {
	t.Parallel()

	ev := householdEvent(t, app.HouseholdMemberRemovedType, member, nil)
	ev.Data = json.RawMessage(`{"userid":`)

	if _, _, err := app.MembershipNotificationIn(ev); err == nil {
		t.Error("an unreadable memberremoved event raised nothing and no error")
	}
}
