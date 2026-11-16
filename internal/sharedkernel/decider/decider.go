// Package decider holds the contract every aggregate implements (ADR-0011):
// Decide turns a command into events, Evolve folds an event into the state.
// Domain packages define their aggregates as Deciders; the platform's
// runtime loads, stamps, stores and forwards what they decide.
package decider

import (
	"time"

	"github.com/google/uuid"
)

// Decider is an aggregate: its initial state, how it decides on a command
// and how its state evolves with each event.
type Decider[S, C, E any] struct {
	Initial S
	Decide  func(S, C) ([]E, error)
	Evolve  func(S, Envelope[E]) S
}

// Envelope is an event as recorded: the event plus what the runtime stamped
// on it once, when it was decided.
type Envelope[E any] struct {
	// ID is the event's ID (UUIDv7).
	ID uuid.UUID
	// StreamID is the aggregate's ID.
	StreamID uuid.UUID
	// StreamVersion is the event's position in its stream, from 1.
	StreamVersion int64
	// Time is when the event occurred.
	Time time.Time
	// HouseholdID is the Household the event belongs to; uuid.Nil for an
	// instance-level event (ADR-0017).
	HouseholdID uuid.UUID
	// ActorID is the User who caused the event; uuid.Nil for an operator
	// run mode.
	ActorID uuid.UUID
	// Event is the domain event.
	Event E
}
