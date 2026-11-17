// Package decidertest tests Deciders as pure given/when/then scenarios:
// given these past events, when this command, then these new events or this
// error.
package decidertest

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

// Scenario is a Decider evolved with past events.
type Scenario[S, C, E any] struct {
	t       testing.TB
	d       decider.Decider[S, C, E]
	state   S
	version int64
}

// Given evolves d's initial state with the past events, numbered from
// stream version 1.
func Given[S, C, E any](t testing.TB, d decider.Decider[S, C, E], past ...E) *Scenario[S, C, E] {
	t.Helper()

	s := &Scenario[S, C, E]{t: t, d: d, state: d.Initial}
	streamID := uuid.New()

	for _, e := range past {
		s.version++
		s.state = d.Evolve(s.state, decider.Envelope[E]{
			ID:            uuid.New(),
			StreamID:      streamID,
			StreamVersion: s.version,
			Time:          time.Now(),
			Event:         e,
		})
	}

	return s
}

// State returns the state the past events produced.
func (s *Scenario[S, C, E]) State() S { return s.state }

// When decides on cmd.
func (s *Scenario[S, C, E]) When(cmd C) *Outcome[E] {
	events, err := s.d.Decide(s.state, cmd)

	return &Outcome[E]{t: s.t, events: events, err: err}
}

// Outcome is what a Decider decided.
type Outcome[E any] struct {
	t      testing.TB
	events []E
	err    error
}

// Then checks that the command was accepted with exactly the events want.
func (o *Outcome[E]) Then(want ...E) {
	o.t.Helper()

	if o.err != nil {
		o.t.Fatalf("decide: unexpected error %v; want events %+v", o.err, want)

		return
	}

	if len(o.events) == 0 && len(want) == 0 {
		return
	}

	if !reflect.DeepEqual(o.events, want) {
		o.t.Errorf("decided events:\n  got  %+v\n  want %+v", o.events, want)
	}
}

// ThenError checks that the command was rejected with an error that is want
// (errors.Is).
func (o *Outcome[E]) ThenError(want error) {
	o.t.Helper()

	if !errors.Is(o.err, want) {
		o.t.Errorf("decide: got events %+v and error %v; want error %v", o.events, o.err, want)
	}
}
