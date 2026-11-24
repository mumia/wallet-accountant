package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// ErrInvitationNotFound is returned for revoking an Invitation that isn't an
// open Invitation of the Household: unknown, another Household's, redeemed,
// revoked or expired.
var ErrInvitationNotFound = errors.New("invitation not found")

// IssuedInvitation is an Invitation to a Household as its issuer sees it,
// once: with its code.
type IssuedInvitation struct {
	ID        uuid.UUID
	Code      string
	ExpiresAt time.Time
}

// OpenInvitation is an Invitation to a Household that can still be
// redeemed, as every member sees it: never with its code.
type OpenInvitation struct {
	ID        uuid.UUID
	IssuedBy  UserRef
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// UserRef names a User: their ID and display name, which is empty for a
// User who has none.
type UserRef struct {
	ID          uuid.UUID
	DisplayName string
}

// InvitationReader reads Invitations to a Household.
type InvitationReader interface {
	// OpenInvitations returns the open Invitations of the Household the
	// transaction is set to, oldest first: not redeemed, not revoked, and
	// expiring after at. Each names its issuer with their display name,
	// read in instance scope.
	OpenInvitations(ctx context.Context, tx *eventstore.Tx, at time.Time) ([]OpenInvitation, error)
	// IsOpen reports whether id is an open Invitation of the Household the
	// transaction is set to, by the same rule as OpenInvitations.
	IsOpen(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, at time.Time) (bool, error)
}

// Invitations issues, lists and revokes Invitations to a Household, which any of its
// members may do (.scratch/join-household).
type Invitations struct {
	households  *Households
	invitations InvitationAggregate
	reader      InvitationReader
}

// NewInvitations returns the Invitations service.
func NewInvitations(households *Households, invitations InvitationAggregate, reader InvitationReader) *Invitations {
	return &Invitations{households: households, invitations: invitations, reader: reader}
}

// Issue issues a new Invitation to the Household as the caller of ctx, who
// must be a member, and returns it with its code, which nobody sees again.
// Each issue is a new Invitation, which expires domain.CodeLifetime after
// it is issued.
func (i *Invitations) Issue(ctx context.Context, household uuid.UUID) (IssuedInvitation, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return IssuedInvitation{}, fmt.Errorf("invitation id: %w", err)
	}

	code := domain.NewInvitationCode()

	var issued IssuedInvitation

	err = i.households.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		res, err := i.invitations.Execute(ctx, tx, id,
			domain.IssueHouseholdInvitation{CodeHash: domain.HashCode(code), At: tx.Now()})
		if err != nil {
			return err //nolint:wrapcheck // wrapped below
		}

		issued = IssuedInvitation{ID: id, Code: code, ExpiresAt: res.State.ExpiresAt}

		return nil
	})
	if err != nil {
		return IssuedInvitation{}, fmt.Errorf("issue invitation: %w", err)
	}

	return issued, nil
}

// ListOpen returns the Household's open Invitations, oldest first, to the
// caller of ctx, who must be a member. Expired ones are left out by the
// transaction's clock, so no expiry event is needed (ADR-0017).
func (i *Invitations) ListOpen(ctx context.Context, household uuid.UUID) ([]OpenInvitation, error) {
	var list []OpenInvitation

	err := i.households.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		var err error
		list, err = i.reader.OpenInvitations(ctx, tx, tx.Now())

		return err //nolint:wrapcheck // the adapter's error
	})
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}

	return list, nil
}

// Revoke revokes the Household's open Invitation as the caller of ctx, who
// must be a member but needn't be its issuer. Its code then gets the same
// error as any invalid one. An Invitation that isn't open gets
// ErrInvitationNotFound: the contract counts an expired one as not open,
// although the Invitation Decider would revoke it.
func (i *Invitations) Revoke(ctx context.Context, household, invitation uuid.UUID) error {
	err := i.households.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		open, err := i.reader.IsOpen(ctx, tx, invitation, tx.Now())
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !open {
			return ErrInvitationNotFound
		}

		_, err = i.invitations.Execute(ctx, tx, invitation, domain.RevokeInvitation{})
		if errors.Is(err, domain.ErrInvitationAlreadyRedeemed) || errors.Is(err, domain.ErrInvitationRevoked) {
			// Redeemed or revoked since IsOpen read it.
			return ErrInvitationNotFound
		}

		return err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}

	return nil
}
