// Package app holds the Households application services and their ports.
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

var (
	// ErrRegistrationRequired is returned for a verified identity that has
	// not redeemed an Invitation, so it is not a User (ADR-0017).
	ErrRegistrationRequired = errors.New("registration required")
	// ErrInvalidInvitation is returned for an Invitation code that is
	// unknown, expired, superseded or already redeemed. The cases are not
	// told apart, so codes can't be probed.
	ErrInvalidInvitation = errors.New("invalid invitation code")
	// ErrAlreadyRegistered is returned for redeeming an Invitation as an
	// identity that is already a User. Fold stores return it when the
	// identity lookup row's unique constraint rejects a second User.
	ErrAlreadyRegistered = errors.New("identity already registered")
	// ErrBootstrapRefused is returned for issuing the bootstrap Invitation
	// once it was redeemed: the instance has its Instance administrator.
	ErrBootstrapRefused = errors.New("the bootstrap Invitation was already redeemed: the instance has an Instance administrator")
)

// User is a registered User as the API sees it.
type User struct {
	ID                      uuid.UUID
	IsInstanceAdministrator bool
}

// Lookups finds Invitations and Users by their lookup rows. All of them are
// in instance scope.
type Lookups interface {
	// InvitationByCode returns the Invitation whose current code has hash.
	InvitationByCode(ctx context.Context, tx *eventstore.Tx, hash domain.CodeHash) (uuid.UUID, bool, error)
	// UserByIdentity returns the User the OIDC identity registered as.
	UserByIdentity(ctx context.Context, tx *eventstore.Tx, id authn.Identity) (User, bool, error)
}

// Users admits identities as Users by Invitation, and resolves identities to
// Users.
type Users struct {
	store       *eventstore.Store
	invitations InvitationAggregate
	users       UserAggregate
	lookups     Lookups
	now         func() time.Time
}

// NewUsers returns the Users service. now is the clock commands are
// timestamped with, which judges Invitation expiry.
func NewUsers(store *eventstore.Store, invitations InvitationAggregate, users UserAggregate, lookups Lookups, now func() time.Time) *Users {
	return &Users{store: store, invitations: invitations, users: users, lookups: lookups, now: now}
}

// Me returns the User the identity belongs to.
func (u *Users) Me(ctx context.Context, id authn.Identity) (User, error) {
	var user User

	err := u.store.InTx(ctx, eventstore.Scope{}, func(ctx context.Context, tx *eventstore.Tx) error {
		found, ok, err := u.lookups.UserByIdentity(ctx, tx, id)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !ok {
			return ErrRegistrationRequired
		}

		user = found

		return nil
	})
	if err != nil {
		return User{}, fmt.Errorf("me: %w", err)
	}

	return user, nil
}

// Register redeems the Invitation code for the identity and registers it as
// a User with the Invitation's role. The Invitation and the User are decided
// in one database transaction, each under its own version check, so two
// concurrent redemptions of one code can't both succeed: the loser retries
// and finds the Invitation redeemed (ADR-0017).
func (u *Users) Register(ctx context.Context, id authn.Identity, code string) (User, error) {
	userID, err := uuid.NewV7()
	if err != nil {
		return User{}, fmt.Errorf("user id: %w", err)
	}

	hash := domain.HashCode(code)

	var user User

	err = u.store.InTx(ctx, eventstore.Scope{ActorID: userID}, func(ctx context.Context, tx *eventstore.Tx) error {
		if _, registered, err := u.lookups.UserByIdentity(ctx, tx, id); err != nil {
			return err //nolint:wrapcheck // the adapter's error
		} else if registered {
			return ErrAlreadyRegistered
		}

		invitationID, ok, err := u.lookups.InvitationByCode(ctx, tx, hash)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !ok {
			return ErrInvalidInvitation
		}

		redeemed, err := u.invitations.Execute(ctx, tx, invitationID,
			domain.RedeemInvitation{CodeHash: hash, UserID: userID, At: u.now()})
		if err != nil {
			return asInvalidInvitation(err)
		}

		registered, err := u.users.Execute(ctx, tx, userID, domain.RegisterUser{
			Issuer:       id.Issuer,
			Subject:      id.Subject,
			InvitationID: invitationID,
			Role:         redeemed.State.Role,
		})
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the runtime
		}

		user = User{ID: userID, IsInstanceAdministrator: registered.State.IsInstanceAdministrator}

		return nil
	})
	if err != nil {
		return User{}, fmt.Errorf("register: %w", err)
	}

	return user, nil
}

// IssueBootstrapInvitation issues the bootstrap Invitation, which grants
// Instance administrator, or reissues it with a new code while it is
// unredeemed. It returns the code, which only its caller ever sees.
func (u *Users) IssueBootstrapInvitation(ctx context.Context) (string, time.Time, error) {
	code, expiresAt, err := u.issue(ctx, domain.BootstrapInvitationID, domain.RoleInstanceAdministrator, u.now().Add(domain.CodeLifetime))
	if errors.Is(err, domain.ErrInvitationAlreadyRedeemed) {
		return "", time.Time{}, ErrBootstrapRefused
	}

	return code, expiresAt, err
}

// IssueInvitation issues a new Invitation granting role and expiring at
// expiresAt, as the operator. Invitations issued from inside the app come
// later (ADR-0017); tests use this to admit several identities.
func (u *Users) IssueInvitation(ctx context.Context, role domain.Role, expiresAt time.Time) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("invitation id: %w", err)
	}

	code, _, err := u.issue(ctx, id, role, expiresAt)

	return code, err
}

func (u *Users) issue(ctx context.Context, id uuid.UUID, role domain.Role, expiresAt time.Time) (string, time.Time, error) {
	code := domain.NewInvitationCode()

	err := u.store.InTx(ctx, eventstore.Scope{}, func(ctx context.Context, tx *eventstore.Tx) error {
		_, err := u.invitations.Execute(ctx, tx, id, domain.IssueInvitation{
			Role:      role,
			CodeHash:  domain.HashCode(code),
			ExpiresAt: expiresAt,
			IssuedBy:  domain.IssuedByOperator,
		})

		return err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("issue invitation: %w", err)
	}

	return code, expiresAt, nil
}

func asInvalidInvitation(err error) error {
	for _, refusal := range []error{
		domain.ErrInvitationNotIssued,
		domain.ErrWrongInvitationCode,
		domain.ErrInvitationExpired,
		domain.ErrInvitationAlreadyRedeemed,
	} {
		if errors.Is(err, refusal) {
			return fmt.Errorf("%w: %w", ErrInvalidInvitation, err)
		}
	}

	return err
}
