package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

// MatchKind is what a Match links.
type MatchKind string

// MatchKindAccountTransfer links the two legs of a Transfer between two of
// the Household's Accounts.
const MatchKindAccountTransfer MatchKind = "account_transfer"

var (
	// ErrMatchAlreadyConfirmed is returned for confirming a Match twice.
	ErrMatchAlreadyConfirmed = errors.New("match already confirmed")
	// ErrMatchNotConfirmed is returned for removing a Match that was never
	// confirmed.
	ErrMatchNotConfirmed = errors.New("match not confirmed")
	// ErrMatchRemoved is returned for removing a Match a second time.
	ErrMatchRemoved = errors.New("match removed")
	// ErrUnknownMatchKind is returned for a Match of a kind there isn't.
	ErrUnknownMatchKind = errors.New("unknown match kind")
	// ErrLegsDontMatch is returned for two Transfer legs that aren't the
	// two sides of one Transfer: the same Transaction twice, amounts that
	// aren't opposite or in different currencies, or counterparts that
	// don't name each other's Account.
	ErrLegsDontMatch = errors.New("the legs aren't the two sides of one transfer")
)

// TransferLeg is a Transfer leg as a Match checks it: the Transaction, its
// Account Posting and the counterpart Account it names.
type TransferLeg struct {
	TransactionID        sharedkernel.ID
	AccountID            sharedkernel.ID
	CounterpartAccountID sharedkernel.ID
	Amount               sharedkernel.Decimal
	Currency             string
}

// MatchLeg is one Transaction a Match links, and the Account it is on.
type MatchLeg struct {
	TransactionID sharedkernel.ID `json:"transactionid"`
	AccountID     sharedkernel.ID `json:"accountid"`
}

// MatchState is a Match's fold. ConfirmedBy and ConfirmedAt are the
// confirming event's actor and time.
type MatchState struct {
	Confirmed   bool
	Removed     bool
	Kind        MatchKind
	Legs        [2]MatchLeg
	ConfirmedBy sharedkernel.ID
	ConfirmedAt time.Time
}

type (
	// MatchCommand is a command to a Match.
	MatchCommand interface{ isMatchCommand() }

	// ConfirmMatch links two Transfer legs of one Transfer.
	ConfirmMatch struct {
		Kind MatchKind
		Legs [2]TransferLeg
	}

	// RemoveMatch removes a Match, when one of its legs is reversed.
	RemoveMatch struct{}
)

func (ConfirmMatch) isMatchCommand() {}
func (RemoveMatch) isMatchCommand()  {}

type (
	// MatchEvent is a Match's event.
	MatchEvent interface{ isMatchEvent() }

	// MatchConfirmed is recorded when a Match is confirmed. Who and when are
	// the event's own.
	MatchConfirmed struct {
		Kind MatchKind   `json:"kind"`
		Legs [2]MatchLeg `json:"legs"`
	}

	// MatchRemoved is recorded when a Match is removed.
	MatchRemoved struct{}
)

func (MatchConfirmed) isMatchEvent() {}
func (MatchRemoved) isMatchEvent()   {}

// Match is the Match Decider: a confirmed link between two records of the
// same money movement (shared kernel), here the two legs of a Transfer.
var Match = decider.Decider[MatchState, MatchCommand, MatchEvent]{
	Decide: decideMatch,
	Evolve: evolveMatch,
}

func decideMatch(s MatchState, c MatchCommand) ([]MatchEvent, error) {
	switch c := c.(type) {
	case ConfirmMatch:
		return confirmMatch(s, c)
	case RemoveMatch:
		return removeMatch(s)
	default:
		return nil, nil
	}
}

// confirmMatch links two legs with opposite amounts in one currency, each
// naming the other's Account as its counterpart.
func confirmMatch(s MatchState, c ConfirmMatch) ([]MatchEvent, error) {
	if s.Confirmed {
		return nil, ErrMatchAlreadyConfirmed
	}

	if c.Kind != MatchKindAccountTransfer {
		return nil, fmt.Errorf("%w: %q", ErrUnknownMatchKind, c.Kind)
	}

	a, b := c.Legs[0], c.Legs[1]

	switch {
	case a.TransactionID == sharedkernel.NilID || b.TransactionID == sharedkernel.NilID ||
		a.TransactionID == b.TransactionID:
		return nil, fmt.Errorf("%w: two different transactions are needed", ErrLegsDontMatch)
	case a.Currency != b.Currency || a.Amount.IsZero() || !a.Amount.Add(b.Amount).IsZero():
		return nil, fmt.Errorf("%w: %s %s and %s %s aren't opposite", ErrLegsDontMatch,
			a.Amount, a.Currency, b.Amount, b.Currency)
	case a.CounterpartAccountID != b.AccountID || b.CounterpartAccountID != a.AccountID:
		return nil, fmt.Errorf("%w: each leg must name the other's account as its counterpart", ErrLegsDontMatch)
	}

	return []MatchEvent{MatchConfirmed{Kind: c.Kind, Legs: [2]MatchLeg{
		{TransactionID: a.TransactionID, AccountID: a.AccountID},
		{TransactionID: b.TransactionID, AccountID: b.AccountID},
	}}}, nil
}

func removeMatch(s MatchState) ([]MatchEvent, error) {
	switch {
	case !s.Confirmed:
		return nil, ErrMatchNotConfirmed
	case s.Removed:
		return nil, ErrMatchRemoved
	}

	return []MatchEvent{MatchRemoved{}}, nil
}

func evolveMatch(s MatchState, e decider.Envelope[MatchEvent]) MatchState {
	switch ev := e.Event.(type) {
	case MatchConfirmed:
		s.Confirmed, s.Kind, s.Legs = true, ev.Kind, ev.Legs
		// Kept to the microsecond, as the fold store keeps times.
		s.ConfirmedBy, s.ConfirmedAt = e.ActorID, e.Time.Truncate(time.Microsecond)
	case MatchRemoved:
		s.Removed = true
	}

	return s
}
