package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

// Role is what redeeming an Invitation makes its User (ADR-0017).
type Role string

// The roles an Invitation can grant.
const (
	// RoleUser grants no instance-level role.
	RoleUser Role = "user"
	// RoleInstanceAdministrator makes the User an Instance administrator.
	RoleInstanceAdministrator Role = "instance_administrator"
)

// IssuedByOperator records that an operator run mode issued an Invitation,
// since there is no User to name as its actor (ADR-0017).
const IssuedByOperator = "operator"

// CodeLifetime is how long an Invitation code is valid after it is issued.
const CodeLifetime = 24 * time.Hour

// BootstrapInvitationID is the fixed, well-known ID of the bootstrap
// Invitation's stream, so only one bootstrap code is ever valid (ADR-0017).
var BootstrapInvitationID = sharedkernel.MustParseID("00000000-0000-7000-8000-000000000001")

var (
	// ErrInvitationAlreadyRedeemed is returned for issuing or redeeming an
	// Invitation that was already redeemed.
	ErrInvitationAlreadyRedeemed = errors.New("invitation already redeemed")
	// ErrInvitationNotIssued is returned for redeeming an Invitation that was
	// never issued.
	ErrInvitationNotIssued = errors.New("invitation not issued")
	// ErrWrongInvitationCode is returned for redeeming with a code that is
	// not the Invitation's current one, for example one a reissue replaced.
	ErrWrongInvitationCode = errors.New("wrong invitation code")
	// ErrInvitationExpired is returned for redeeming at or after the code's
	// expiry.
	ErrInvitationExpired = errors.New("invitation expired")
)

// CodeHash is the SHA-256 hash of an Invitation code, hex-encoded. Only the
// hash is stored: 256 random bits need no slower hash.
type CodeHash string

// NewInvitationCode returns a new Invitation code of 256 random bits.
func NewInvitationCode() string {
	var b [32]byte

	_, _ = rand.Read(b[:]) // crypto/rand never fails

	return base64.RawURLEncoding.EncodeToString(b[:])
}

// HashCode returns the hash of an Invitation code.
func HashCode(code string) CodeHash {
	sum := sha256.Sum256([]byte(code))

	return CodeHash(hex.EncodeToString(sum[:]))
}

// InvitationState is an Invitation's fold.
type InvitationState struct {
	Issued     bool
	Role       Role
	CodeHash   CodeHash
	ExpiresAt  time.Time
	RedeemedBy sharedkernel.ID
	RedeemedAt time.Time
}

// Redeemed reports whether the Invitation was redeemed.
func (s InvitationState) Redeemed() bool { return s.RedeemedBy != sharedkernel.NilID }

type (
	// InvitationCommand is a command to an Invitation.
	InvitationCommand interface{ isInvitationCommand() }

	// IssueInvitation issues the Invitation, or reissues it with a new code
	// and expiry while it is unredeemed.
	IssueInvitation struct {
		Role      Role
		CodeHash  CodeHash
		ExpiresAt time.Time
		IssuedBy  string
	}
)

// RedeemInvitation redeems the Invitation for a new User. At is the
// command's timestamp, which judges expiry, so no expiry event is needed.
type RedeemInvitation struct {
	CodeHash CodeHash
	UserID   sharedkernel.ID
	At       time.Time
}

func (IssueInvitation) isInvitationCommand()  {}
func (RedeemInvitation) isInvitationCommand() {}

type (
	// InvitationEvent is an Invitation's event.
	InvitationEvent interface{ isInvitationEvent() }

	// InvitationIssued is recorded when an Invitation is first issued.
	InvitationIssued struct {
		Role      Role      `json:"role"`
		CodeHash  CodeHash  `json:"codehash"`
		ExpiresAt time.Time `json:"expiresat"`
		IssuedBy  string    `json:"issuedby,omitempty"`
	}

	// InvitationReissued is recorded when an unredeemed Invitation is issued
	// again: the new code replaces the old one, which no longer works.
	InvitationReissued struct {
		CodeHash  CodeHash  `json:"codehash"`
		ExpiresAt time.Time `json:"expiresat"`
		IssuedBy  string    `json:"issuedby,omitempty"`
	}

	// InvitationRedeemed is recorded when a User redeems the Invitation.
	InvitationRedeemed struct {
		UserID sharedkernel.ID `json:"userid"`
	}
)

func (InvitationIssued) isInvitationEvent()   {}
func (InvitationReissued) isInvitationEvent() {}
func (InvitationRedeemed) isInvitationEvent() {}

// Invitation is the Invitation Decider (instance scope, ADR-0017).
var Invitation = decider.Decider[InvitationState, InvitationCommand, InvitationEvent]{
	Decide: decideInvitation,
	Evolve: evolveInvitation,
}

func decideInvitation(s InvitationState, c InvitationCommand) ([]InvitationEvent, error) {
	switch c := c.(type) {
	case IssueInvitation:
		if s.Redeemed() {
			return nil, ErrInvitationAlreadyRedeemed
		}

		if s.Issued {
			return []InvitationEvent{InvitationReissued{CodeHash: c.CodeHash, ExpiresAt: c.ExpiresAt, IssuedBy: c.IssuedBy}}, nil
		}

		return []InvitationEvent{InvitationIssued(c)}, nil
	case RedeemInvitation:
		switch {
		case !s.Issued:
			return nil, ErrInvitationNotIssued
		case subtle.ConstantTimeCompare([]byte(s.CodeHash), []byte(c.CodeHash)) != 1:
			return nil, ErrWrongInvitationCode
		case s.Redeemed():
			return nil, ErrInvitationAlreadyRedeemed
		case !c.At.Before(s.ExpiresAt):
			return nil, ErrInvitationExpired
		}

		return []InvitationEvent{InvitationRedeemed{UserID: c.UserID}}, nil
	default:
		return nil, nil
	}
}

func evolveInvitation(s InvitationState, e decider.Envelope[InvitationEvent]) InvitationState {
	switch ev := e.Event.(type) {
	case InvitationIssued:
		s.Issued, s.Role, s.CodeHash, s.ExpiresAt = true, ev.Role, ev.CodeHash, ev.ExpiresAt
	case InvitationReissued:
		s.CodeHash, s.ExpiresAt = ev.CodeHash, ev.ExpiresAt
	case InvitationRedeemed:
		s.RedeemedBy, s.RedeemedAt = ev.UserID, e.Time
	}

	return s
}
