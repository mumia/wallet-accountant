package domain

import (
	"errors"
	"strings"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

var (
	// ErrUserAlreadyRegistered is returned for registering a User twice.
	ErrUserAlreadyRegistered = errors.New("user already registered")
	// ErrDisplayNameRequired is returned for a blank display name.
	ErrDisplayNameRequired = errors.New("display name required")
)

// UserState is a User's fold.
type UserState struct {
	Registered              bool
	Issuer                  string
	Subject                 string
	InvitationID            sharedkernel.ID
	IsInstanceAdministrator bool
	// DisplayName is what other members see; empty for a User who
	// registered before display names existed and hasn't set one.
	DisplayName string
}

type (
	// UserCommand is a command to a User.
	UserCommand interface{ isUserCommand() }

	// RegisterUser registers the OIDC identity as a User, with the role of
	// the Invitation it redeemed and the display name it chose.
	RegisterUser struct {
		Issuer       string
		Subject      string
		InvitationID sharedkernel.ID
		Role         Role
		DisplayName  string
	}
)

func (RegisterUser) isUserCommand() {}

type (
	// UserEvent is a User's event.
	UserEvent interface{ isUserEvent() }

	// UserRegistered is recorded when an identity becomes a User by
	// redeeming an Invitation.
	UserRegistered struct {
		Issuer       string          `json:"issuer"`
		Subject      string          `json:"subject"`
		InvitationID sharedkernel.ID `json:"invitationid"`
		Role         Role            `json:"role"`
		// DisplayName is absent on events recorded before display names
		// existed.
		DisplayName string `json:"displayname,omitempty"`
	}
)

func (UserRegistered) isUserEvent() {}

// User is the User Decider (instance scope, ADR-0017).
var User = decider.Decider[UserState, UserCommand, UserEvent]{
	Decide: decideUser,
	Evolve: evolveUser,
}

func decideUser(s UserState, c UserCommand) ([]UserEvent, error) {
	switch c := c.(type) {
	case RegisterUser:
		if s.Registered {
			return nil, ErrUserAlreadyRegistered
		}

		name := strings.TrimSpace(c.DisplayName)
		if name == "" {
			return nil, ErrDisplayNameRequired
		}

		c.DisplayName = name

		return []UserEvent{UserRegistered(c)}, nil
	default:
		return nil, nil
	}
}

func evolveUser(s UserState, e decider.Envelope[UserEvent]) UserState {
	if ev, ok := e.Event.(UserRegistered); ok {
		s.Registered = true
		s.Issuer, s.Subject, s.InvitationID = ev.Issuer, ev.Subject, ev.InvitationID
		s.IsInstanceAdministrator = ev.Role == RoleInstanceAdministrator
		s.DisplayName = ev.DisplayName
	}

	return s
}
