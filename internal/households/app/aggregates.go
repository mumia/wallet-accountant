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

	UserStreamType     = "households.user"
	UserRegisteredType = "io.walletaccountant.households.user.registered.v1"
)

type (
	// InvitationAggregate is the Invitation Decider wired to the runtime.
	InvitationAggregate = eventstore.Aggregate[domain.InvitationState, domain.InvitationCommand, domain.InvitationEvent]
	// UserAggregate is the User Decider wired to the runtime.
	UserAggregate = eventstore.Aggregate[domain.UserState, domain.UserCommand, domain.UserEvent]
)

// NewInvitationAggregate wires the Invitation Decider to its fold store.
func NewInvitationAggregate(folds eventstore.FoldStore[domain.InvitationState]) InvitationAggregate {
	codec := eventstore.NewCodec[domain.InvitationEvent](Source)
	eventstore.Register[domain.InvitationEvent, domain.InvitationIssued](codec, InvitationIssuedType)
	eventstore.Register[domain.InvitationEvent, domain.InvitationReissued](codec, InvitationReissuedType)
	eventstore.Register[domain.InvitationEvent, domain.InvitationRedeemed](codec, InvitationRedeemedType)

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

	return UserAggregate{
		StreamType:        UserStreamType,
		Decider:           domain.User,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
	}
}
