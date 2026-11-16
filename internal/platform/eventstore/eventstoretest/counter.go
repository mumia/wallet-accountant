// Package eventstoretest gives platform integration tests a small aggregate,
// a counter, with a Household-scoped fold table of its own, so the Decider
// runtime, rebuild and event forwarding can be exercised against PostgreSQL.
package eventstoretest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

// Event types of the counter.
const (
	StreamType = "test.counter"
	AddedType  = "io.walletaccountant.test.counter.added.v1"
)

// ErrOverLimit is the counter's only rule: its value stays at most 100.
var ErrOverLimit = errors.New("counter over its limit")

type (
	// State is the counter's fold.
	State struct {
		Value int64
		// Note is whatever the last Added event's Note was after a round
		// trip through the codec: never set, because Note is not encoded.
		Note string
		// Events counts the events evolved.
		Events int64
	}
	// Add adds N, recording Note on the event.
	Add struct {
		N    int64
		Note string
	}
	// Event is the counter's event sum type.
	Event interface{ isCounterEvent() }
	// Added is recorded by Add.
	Added struct {
		N    int64  `json:"n"`
		Note string `json:"-"`
	}
)

func (Added) isCounterEvent() {}

// Decider is the counter.
var Decider = decider.Decider[State, Add, Event]{
	Decide: func(s State, c Add) ([]Event, error) {
		if s.Value+c.N > 100 {
			return nil, ErrOverLimit
		}

		if c.N == 0 {
			return nil, nil
		}

		return []Event{Added(c)}, nil
	},
	Evolve: func(s State, e decider.Envelope[Event]) State {
		if a, ok := e.Event.(Added); ok {
			s.Value += a.N
			s.Note = a.Note
		}

		s.Events++

		return s
	},
}

// Codec encodes the counter's events.
func Codec() *eventstore.Codec[Event] {
	c := eventstore.NewCodec[Event]("/test")
	eventstore.Register[Event, Added](c, AddedType)

	return c
}

// Counter is the counter aggregate wired to its fold table.
type Counter struct {
	eventstore.Aggregate[State, Add, Event]

	Folds *Folds
}

// New creates the counter's fold table (as wa_owner, through ownerURL) and
// returns the aggregate at fold schema version 1.
func New(t testing.TB, ownerURL string) *Counter {
	t.Helper()

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, ownerURL)
	if err != nil {
		t.Fatalf("connect as owner: %v", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, schema); err != nil {
		t.Fatalf("counter fold table: %v", err)
	}

	folds := &Folds{}

	return &Counter{
		Aggregate: eventstore.Aggregate[State, Add, Event]{
			StreamType:        StreamType,
			Decider:           Decider,
			Codec:             Codec(),
			Folds:             folds,
			FoldSchemaVersion: 1,
		},
		Folds: folds,
	}
}

const schema = `
CREATE TABLE IF NOT EXISTS test_counters (
    id             uuid   PRIMARY KEY,
    household_id   uuid,
    value          bigint NOT NULL,
    note           text   NOT NULL,
    events         bigint NOT NULL,
    version        bigint NOT NULL,
    schema_version int    NOT NULL
);
ALTER TABLE test_counters ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_counters FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS household_or_instance ON test_counters;
CREATE POLICY household_or_instance ON test_counters
    USING ((household_id IS NULL AND NULLIF(current_setting('app.household_id', true), '') IS NULL)
        OR household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK ((household_id IS NULL AND NULLIF(current_setting('app.household_id', true), '') IS NULL)
        OR household_id = NULLIF(current_setting('app.household_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE ON test_counters TO wa_app;
`

// Folds is the counter's FoldStore. Loads counts every Load.
type Folds struct {
	Loads atomic.Int64

	mu   sync.Mutex
	race func()
}

// OnNextLoad runs fn once, right after the next Load has read the fold, to
// let a concurrent command win the race for the stream.
func (f *Folds) OnNextLoad(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.race = fn
}

// Load implements eventstore.FoldStore.
func (f *Folds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[State], bool, error) {
	f.Loads.Add(1)

	defer func() {
		f.mu.Lock()
		race := f.race
		f.race = nil
		f.mu.Unlock()

		if race != nil {
			race()
		}
	}()

	var fold eventstore.Fold[State]

	err := tx.QueryRow(ctx, `SELECT value, note, events, version, schema_version FROM test_counters WHERE id = $1`, id).
		Scan(&fold.State.Value, &fold.State.Note, &fold.State.Events, &fold.Version, &fold.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[State]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[State]{}, false, fmt.Errorf("load counter: %w", err)
	}

	return fold, true, nil
}

// Save implements eventstore.FoldStore.
func (*Folds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, fold eventstore.Fold[State], expected int64) error {
	var household *uuid.UUID
	if h := tx.HouseholdID(); h != uuid.Nil {
		household = &h
	}

	q := `UPDATE test_counters SET value = $2, note = $3, events = $4, version = $5, schema_version = $6
	      WHERE id = $1 AND version = $7`
	if expected == 0 {
		q = `INSERT INTO test_counters (id, value, note, events, version, schema_version, household_id)
		     VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING`
	}

	last := any(expected)
	if expected == 0 {
		last = household
	}

	tag, err := tx.Exec(ctx, q, id, fold.State.Value, fold.State.Note, fold.State.Events, fold.Version, fold.SchemaVersion, last)
	if err != nil {
		return fmt.Errorf("save counter: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	return nil
}
