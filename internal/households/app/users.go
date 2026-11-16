// Package app holds the Households application services and their ports.
package app

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/platform/authn"
)

// ErrRegistrationRequired is returned for a verified identity that has not
// redeemed an Invitation, so it is not a User (ADR-0017).
var ErrRegistrationRequired = errors.New("registration required")

// User is a registered User as the API sees it.
type User struct {
	ID                      uuid.UUID
	IsInstanceAdministrator bool
}

// Users resolves verified identities to Users.
type Users struct{}

// NewUsers returns the Users service.
func NewUsers() *Users {
	return &Users{}
}

// Me returns the User the identity belongs to. Until Invitations can be
// redeemed no User exists, so every identity is unregistered.
func (*Users) Me(_ context.Context, _ authn.Identity) (User, error) {
	return User{}, ErrRegistrationRequired
}
