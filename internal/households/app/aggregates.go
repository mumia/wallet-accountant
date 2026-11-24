package app

import (
	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// Stream types and event types of the Households aggregates (ADR-0015,
// ADR-0017).
const (
	Source = "/households"

	InvitationStreamType   = "households.invitation"
	InvitationIssuedType   = "io.walletaccountant.households.invitation.issued.v1"
	InvitationReissuedType = "io.walletaccountant.households.invitation.reissued.v1"
	InvitationRedeemedType = "io.walletaccountant.households.invitation.redeemed.v1"
	InvitationRevokedType  = "io.walletaccountant.households.invitation.revoked.v1"

	UserStreamType     = "households.user"
	UserRegisteredType = "io.walletaccountant.households.user.registered.v1"
	UserRenamedType    = "io.walletaccountant.households.user.renamed.v1"

	HouseholdStreamType      = "households.household"
	HouseholdCreatedType     = "io.walletaccountant.households.household.created.v1"
	HouseholdMemberAddedType = "io.walletaccountant.households.household.memberadded.v1"

	NotificationStreamType = "households.notification"
	NotificationRaisedType = "io.walletaccountant.households.notification.raised.v1"
	NotificationReadType   = "io.walletaccountant.households.notification.read.v1"
)

// NotificationAggregate is the Notification Decider wired to the runtime.
type NotificationAggregate = eventstore.Aggregate[domain.NotificationState, domain.NotificationCommand, domain.NotificationEvent]

// NewNotificationAggregate wires the Notification Decider to its fold store.
func NewNotificationAggregate(folds eventstore.FoldStore[domain.NotificationState]) NotificationAggregate {
	codec := eventstore.NewCodec[domain.NotificationEvent](Source)
	eventstore.Register[domain.NotificationEvent, domain.NotificationRaised](codec, NotificationRaisedType)
	eventstore.Register[domain.NotificationEvent, domain.NotificationRead](codec, NotificationReadType)

	return NotificationAggregate{
		StreamType:        NotificationStreamType,
		Decider:           domain.Notification,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
	}
}

type (
	// InvitationAggregate is the Invitation Decider wired to the runtime.
	InvitationAggregate = eventstore.Aggregate[domain.InvitationState, domain.InvitationCommand, domain.InvitationEvent]
	// UserAggregate is the User Decider wired to the runtime.
	UserAggregate = eventstore.Aggregate[domain.UserState, domain.UserCommand, domain.UserEvent]
	// HouseholdAggregate is the Household Decider wired to the runtime.
	HouseholdAggregate = eventstore.Aggregate[domain.HouseholdState, domain.HouseholdCommand, domain.HouseholdEvent]
)

// NewInvitationAggregate wires the Invitation Decider to its fold store.
func NewInvitationAggregate(folds eventstore.FoldStore[domain.InvitationState]) InvitationAggregate {
	codec := eventstore.NewCodec[domain.InvitationEvent](Source)
	eventstore.Register[domain.InvitationEvent, domain.InvitationIssued](codec, InvitationIssuedType)
	eventstore.Register[domain.InvitationEvent, domain.InvitationReissued](codec, InvitationReissuedType)
	eventstore.Register[domain.InvitationEvent, domain.InvitationRedeemed](codec, InvitationRedeemedType)
	eventstore.Register[domain.InvitationEvent, domain.InvitationRevoked](codec, InvitationRevokedType)

	return InvitationAggregate{
		StreamType:        InvitationStreamType,
		Decider:           domain.Invitation,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
	}
}

// NewUserAggregate wires the User Decider to its fold store.
func NewUserAggregate(folds eventstore.FoldStore[domain.UserState]) UserAggregate {
	codec := eventstore.NewCodec[domain.UserEvent](Source)
	eventstore.Register[domain.UserEvent, domain.UserRegistered](codec, UserRegisteredType)
	eventstore.Register[domain.UserEvent, domain.UserRenamed](codec, UserRenamedType)

	return UserAggregate{
		StreamType:        UserStreamType,
		Decider:           domain.User,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 2, // 2: display name
	}
}

// NewHouseholdAggregate wires the Household Decider to its fold store.
func NewHouseholdAggregate(folds eventstore.FoldStore[domain.HouseholdState]) HouseholdAggregate {
	codec := eventstore.NewCodec[domain.HouseholdEvent](Source)
	eventstore.Register[domain.HouseholdEvent, domain.HouseholdCreated](codec, HouseholdCreatedType)
	eventstore.Register[domain.HouseholdEvent, domain.HouseholdMemberAdded](codec, HouseholdMemberAddedType)

	return HouseholdAggregate{
		StreamType:        HouseholdStreamType,
		Decider:           domain.Household,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
	}
}
