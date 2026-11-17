package eventstore

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/propagation"

	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

// Fold is an aggregate's stored state: its current state, the stream version
// it reflects and the fold schema version it was built with.
type Fold[S any] struct {
	State         S
	Version       int64
	SchemaVersion int
}

// FoldStore stores one aggregate's folds (and any lookup rows) in its own
// tables. It is the port each context's PostgreSQL adapter implements.
type FoldStore[S any] interface {
	// Load returns the stored fold, or false when the stream has none.
	Load(ctx context.Context, tx *Tx, streamID uuid.UUID) (Fold[S], bool, error)
	// Save stores fold if the stored fold is still at expectedVersion (0:
	// none stored), and returns ErrVersionConflict otherwise.
	Save(ctx context.Context, tx *Tx, streamID uuid.UUID, fold Fold[S], expectedVersion int64) error
}

// Aggregate is a Decider wired to its codec and fold store.
type Aggregate[S, C, E any] struct {
	// StreamType names the aggregate's streams ("<context>.<aggregate>").
	StreamType string
	Decider    decider.Decider[S, C, E]
	Codec      *Codec[E]
	Folds      FoldStore[S]
	// FoldSchemaVersion is the version of the fold's structure. A stored
	// fold built with an older one is rebuilt from its stream.
	FoldSchemaVersion int
}

// Result is what a command did: the new state and version, and the events
// it recorded (none when the Decider decided nothing).
type Result[S, E any] struct {
	State   S
	Version int64
	Events  []decider.Envelope[E]
}

// Execute runs cmd against the stream in tx: it loads the fold and its
// version, decides, stamps each event once, encodes and decodes it so the
// live fold is exactly what a replay produces, evolves, appends the events
// and saves the fold if its version is unchanged. A conflict comes back as
// ErrVersionConflict, which Store.InTx retries.
func (a Aggregate[S, C, E]) Execute(ctx context.Context, tx *Tx, streamID uuid.UUID, cmd C) (Result[S, E], error) {
	fold, stored, err := a.load(ctx, tx, streamID)
	if err != nil {
		return Result[S, E]{}, err
	}

	decided, err := a.Decider.Decide(fold.State, cmd)
	if err != nil {
		return Result[S, E]{}, err //nolint:wrapcheck // the domain's own error
	}

	res := Result[S, E]{State: fold.State, Version: fold.Version}
	if len(decided) == 0 {
		return res, nil
	}

	trace := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, trace)

	recorded := make([]CloudEvent, 0, len(decided))

	for _, event := range decided {
		id, err := uuid.NewV7()
		if err != nil {
			return Result[S, E]{}, fmt.Errorf("event id: %w", err)
		}

		res.Version++

		ce, err := a.Codec.Encode(CloudEvent{
			ID:            id,
			Subject:       streamID,
			Time:          tx.now().UTC(),
			HouseholdID:   tx.scope.HouseholdID,
			StreamType:    a.StreamType,
			StreamVersion: res.Version,
			ActorID:       tx.scope.ActorID,
			CorrelationID: tx.scope.CorrelationID,
			CausationID:   tx.scope.CausationID,
			TraceParent:   trace.Get("traceparent"),
			TraceState:    trace.Get("tracestate"),
		}, event)
		if err != nil {
			return Result[S, E]{}, err
		}

		env, err := a.Codec.Decode(ce)
		if err != nil {
			return Result[S, E]{}, err
		}

		res.State = a.Decider.Evolve(res.State, env)
		res.Events = append(res.Events, env)
		recorded = append(recorded, ce)
	}

	if err := appendEvents(ctx, tx.Tx, recorded); err != nil {
		return Result[S, E]{}, asConflict(err)
	}

	next := Fold[S]{State: res.State, Version: res.Version, SchemaVersion: a.FoldSchemaVersion}
	if err := a.Folds.Save(ctx, tx, streamID, next, stored); err != nil {
		return Result[S, E]{}, fmt.Errorf("save %s fold %s: %w", a.StreamType, streamID, err)
	}

	return res, nil
}

// load returns the current fold and the version of the stored row, which
// the save is conditional on. A fold built with an older schema version is
// rebuilt from the stream first.
func (a Aggregate[S, C, E]) load(ctx context.Context, tx *Tx, streamID uuid.UUID) (Fold[S], int64, error) {
	fold, found, err := a.Folds.Load(ctx, tx, streamID)
	if err != nil {
		return Fold[S]{}, 0, fmt.Errorf("load %s fold %s: %w", a.StreamType, streamID, err)
	}

	if !found {
		return Fold[S]{State: a.Decider.Initial}, 0, nil
	}

	if fold.SchemaVersion >= a.FoldSchemaVersion {
		return fold, fold.Version, nil
	}

	events, err := ReadStream(ctx, tx, streamID)
	if err != nil {
		return Fold[S]{}, 0, err
	}

	rebuilt, err := a.replay(events)
	if err != nil {
		return Fold[S]{}, 0, err
	}

	return rebuilt, fold.Version, nil
}

// replay folds a stream's events through the codec, upcasting old ones.
func (a Aggregate[S, C, E]) replay(events []CloudEvent) (Fold[S], error) {
	fold := Fold[S]{State: a.Decider.Initial, SchemaVersion: a.FoldSchemaVersion}

	for _, ce := range events {
		env, err := a.Codec.Decode(ce)
		if err != nil {
			return Fold[S]{}, fmt.Errorf("replay %s %s: %w", a.StreamType, ce.Subject, err)
		}

		fold.State = a.Decider.Evolve(fold.State, env)
		fold.Version = ce.StreamVersion
	}

	return fold, nil
}

// RebuildTarget lets Rebuild rebuild this aggregate's folds.
func (a Aggregate[S, C, E]) RebuildTarget() RebuildTarget {
	return RebuildTarget{
		StreamType: a.StreamType,
		Rebuild: func(ctx context.Context, tx *Tx, streamID uuid.UUID, events []CloudEvent) error {
			fold, err := a.replay(events)
			if err != nil {
				return err
			}

			stored, found, err := a.Folds.Load(ctx, tx, streamID)
			if err != nil {
				return fmt.Errorf("load %s fold %s: %w", a.StreamType, streamID, err)
			}

			var expected int64
			if found {
				expected = stored.Version
			}

			if expected > fold.Version {
				// A command recorded more events since the stream was read.
				return fmt.Errorf("%w: %s %s moved on while rebuilding", ErrVersionConflict, a.StreamType, streamID)
			}

			if err := a.Folds.Save(ctx, tx, streamID, fold, expected); err != nil {
				return fmt.Errorf("save %s fold %s: %w", a.StreamType, streamID, err)
			}

			return nil
		},
	}
}
