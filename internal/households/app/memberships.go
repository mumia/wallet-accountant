package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// ErrAlreadyAMember is returned for joining a Household the User already
// belongs to with a code that could be redeemed. The transaction is rolled
// back, so the code stays unused.
var ErrAlreadyAMember = errors.New("already a member of the household")

// Join redeems an Invitation to a Household for the User the identity
// belongs to, making them a member, and returns the Household joined. It is
// the Join Household command (consistency map), in one database transaction:
// the caller and the code are resolved in instance scope, then the
// transaction is set to the Invitation's Household, the member is added and
// only then the Invitation redeemed, so a refusal leaves the code unused.
// An unregistered identity gets ErrRegistrationRequired; any code that
// can't be redeemed, including one that names no Household, gets
// ErrInvalidInvitation.
func (u *Users) Join(ctx context.Context, id authn.Identity, code string) (Household, error) {
	hash := domain.HashCode(code)

	var joined Household

	err := u.store.InTx(ctx, eventstore.Scope{}, func(ctx context.Context, tx *eventstore.Tx) error {
		user, ok, err := u.lookups.UserByIdentity(ctx, tx, id)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !ok {
			return ErrRegistrationRequired
		}

		found, ok, err := u.lookups.InvitationByCode(ctx, tx, hash)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !ok || found.HouseholdID == uuid.Nil {
			return ErrInvalidInvitation
		}

		tx.SetActor(user.ID)

		joined, err = u.join(ctx, tx, user.ID, hash, found)

		return err
	})
	if err != nil {
		return Household{}, fmt.Errorf("join household: %w", err)
	}

	return joined, nil
}

// join sets the transaction to the Invitation's Household, adds the User as
// a member, then redeems the Invitation. The transaction must be in
// instance scope, with the User as its actor. A code that can't be redeemed
// gets ErrInvalidInvitation even from a member of its Household, so the
// refusal never reveals a revoked, expired or redeemed code; a member with
// a code that can be redeemed gets ErrAlreadyAMember, and the caller's
// rollback leaves it unused.
func (u *Users) join(ctx context.Context, tx *eventstore.Tx, user uuid.UUID, hash domain.CodeHash, code InvitationCode,
) (Household, error) {
	if err := tx.SetHousehold(ctx, code.HouseholdID); err != nil {
		return Household{}, err //nolint:wrapcheck // already wrapped by the runtime
	}

	added, err := u.households.Execute(ctx, tx, code.HouseholdID,
		domain.AddMember{UserID: user, InvitationID: code.InvitationID})

	member := errors.Is(err, domain.ErrAlreadyAMember)

	switch {
	case member:
		// Redeemed below only to check the code, which a refusal of an
		// unusable one must not reveal; the transaction is rolled back.
	case errors.Is(err, domain.ErrHouseholdNotCreated):
		return Household{}, fmt.Errorf("%w: %w", ErrInvalidInvitation, err)
	case err != nil:
		return Household{}, err //nolint:wrapcheck // already wrapped by the runtime
	}

	if _, err := u.invitations.Execute(ctx, tx, code.InvitationID,
		domain.RedeemInvitation{CodeHash: hash, UserID: user, At: tx.Now()}); err != nil {
		return Household{}, asInvalidInvitation(err)
	}

	if member {
		return Household{}, ErrAlreadyAMember
	}

	return Household{ID: code.HouseholdID, Name: added.State.Name, BaseCurrency: added.State.BaseCurrency}, nil
}
