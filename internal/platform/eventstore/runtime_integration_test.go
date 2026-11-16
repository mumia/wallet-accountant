//go:build integration

package eventstore_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore/eventstoretest"
)

type env struct {
	urls    databasetest.URLs
	app     *pgxpool.Pool
	reader  *pgxpool.Pool
	store   *eventstore.Store
	counter *eventstoretest.Counter
}

var now = time.Date(2026, 11, 16, 9, 30, 0, 0, time.UTC)

func setup(t *testing.T, opts ...eventstore.Option) *env {
	t.Helper()

	urls := databasetest.New(t)
	e := &env{urls: urls, app: pool(t, urls.App), reader: pool(t, urls.Reader)}
	e.store = eventstore.NewStore(e.app, append([]eventstore.Option{eventstore.WithClock(func() time.Time { return now })}, opts...)...)
	e.counter = eventstoretest.New(t, urls.Owner)

	return e
}

func pool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()

	p, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(p.Close)

	return p
}

func (e *env) add(t *testing.T, scope eventstore.Scope, stream uuid.UUID, n int64) eventstore.Result[eventstoretest.State, eventstoretest.Event] {
	t.Helper()

	var res eventstore.Result[eventstoretest.State, eventstoretest.Event]

	err := e.store.InTx(t.Context(), scope, func(ctx context.Context, tx *eventstore.Tx) error {
		var err error
		res, err = e.counter.Execute(ctx, tx, stream, eventstoretest.Add{N: n, Note: "lost in encoding"})

		return err
	})
	if err != nil {
		t.Fatalf("add %d: %v", n, err)
	}

	return res
}

func (e *env) load(t *testing.T, scope eventstore.Scope, stream uuid.UUID) (eventstore.Fold[eventstoretest.State], bool) {
	t.Helper()

	var (
		fold  eventstore.Fold[eventstoretest.State]
		found bool
	)

	err := e.store.InTx(t.Context(), scope, func(ctx context.Context, tx *eventstore.Tx) error {
		var err error
		fold, found, err = e.counter.Folds.Load(ctx, tx, stream)

		return err
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	return fold, found
}

func TestACommandAppendsItsEventsAndSavesTheFold(t *testing.T) {
	t.Parallel()
	e := setup(t)
	household := uuid.New()
	scope := eventstore.Scope{HouseholdID: household}
	stream := uuid.Must(uuid.NewV7())

	e.add(t, scope, stream, 5)
	res := e.add(t, scope, stream, 7)

	if res.State.Value != 12 || res.Version != 2 {
		t.Errorf("result = value %d at version %d, want 12 at version 2", res.State.Value, res.Version)
	}

	fold, found := e.load(t, scope, stream)
	if !found || fold.State.Value != 12 || fold.Version != 2 || fold.SchemaVersion != 1 {
		t.Errorf("stored fold = %+v (found %v), want value 12 at version 2, schema version 1", fold, found)
	}

	events, err := eventstore.ReadStream(t.Context(), e.reader, stream)
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 2 || events[0].StreamVersion != 1 || events[1].StreamVersion != 2 {
		t.Fatalf("stream = %+v, want versions 1 and 2", events)
	}

	if events[1].Type != eventstoretest.AddedType || string(events[1].Data) != `{"n": 7}` && string(events[1].Data) != `{"n":7}` {
		t.Errorf("second event = %s %s, want %s {\"n\":7}", events[1].Type, events[1].Data, eventstoretest.AddedType)
	}
}

func TestTheLiveFoldIsWhatAReplayProduces(t *testing.T) {
	t.Parallel()
	e := setup(t)
	scope := eventstore.Scope{HouseholdID: uuid.New()}

	res := e.add(t, scope, uuid.Must(uuid.NewV7()), 1)

	// Note is not encoded, so a replay never sees it; neither may the live fold.
	if res.State.Note != "" {
		t.Errorf("live fold evolved with the event before encoding: note = %q", res.State.Note)
	}
}

func TestEventsAreStampedOnce(t *testing.T) {
	t.Parallel()

	spans := sdktrace.NewTracerProvider()
	ctx, span := spans.Tracer("test").Start(context.Background(), "command")

	e := setup(t)
	scope := eventstore.Scope{HouseholdID: uuid.New(), ActorID: uuid.New(), CorrelationID: uuid.New(), CausationID: uuid.New()}
	stream := uuid.Must(uuid.NewV7())

	err := e.store.InTx(ctx, scope, func(ctx context.Context, tx *eventstore.Tx) error {
		_, err := e.counter.Execute(ctx, tx, stream, eventstoretest.Add{N: 1})

		return err
	})
	span.End()

	if err != nil {
		t.Fatal(err)
	}

	events, err := eventstore.ReadStream(t.Context(), e.reader, stream)
	if err != nil || len(events) != 1 {
		t.Fatalf("stream: %v, %d events", err, len(events))
	}

	ev := events[0]
	if ev.ID.Version() != 7 {
		t.Errorf("event ID %s is not a UUIDv7", ev.ID)
	}

	if !ev.Time.Equal(now) {
		t.Errorf("time = %v, want the clock's %v", ev.Time, now)
	}

	if ev.Subject != stream || ev.StreamType != eventstoretest.StreamType || ev.Source != "/test" {
		t.Errorf("subject %s, streamtype %q, source %q", ev.Subject, ev.StreamType, ev.Source)
	}

	if ev.HouseholdID != scope.HouseholdID || ev.ActorID != scope.ActorID ||
		ev.CorrelationID != scope.CorrelationID || ev.CausationID != scope.CausationID {
		t.Errorf("event %+v does not carry the scope %+v", ev, scope)
	}

	want := "00-" + span.SpanContext().TraceID().String() + "-" + span.SpanContext().SpanID().String() + "-01"
	if ev.TraceParent != want {
		t.Errorf("traceparent = %q, want %q", ev.TraceParent, want)
	}
}

func TestARejectedCommandRecordsNothing(t *testing.T) {
	t.Parallel()
	e := setup(t)
	scope := eventstore.Scope{HouseholdID: uuid.New()}
	stream := uuid.Must(uuid.NewV7())

	e.add(t, scope, stream, 100)

	err := e.store.InTx(t.Context(), scope, func(ctx context.Context, tx *eventstore.Tx) error {
		_, err := e.counter.Execute(ctx, tx, stream, eventstoretest.Add{N: 1})

		return err
	})
	if !errors.Is(err, eventstoretest.ErrOverLimit) {
		t.Fatalf("err = %v, want ErrOverLimit", err)
	}

	if loads := e.counter.Folds.Loads.Load(); loads != 2 {
		t.Errorf("a rejected command was retried: %d loads, want 2", loads)
	}

	if events, _ := eventstore.ReadStream(t.Context(), e.reader, stream); len(events) != 1 {
		t.Errorf("stream has %d events, want 1", len(events))
	}
}

func TestAVersionConflictRetriesTheCommand(t *testing.T) {
	t.Parallel()
	e := setup(t)
	scope := eventstore.Scope{HouseholdID: uuid.New()}

	for _, c := range []struct {
		name     string
		existing bool
		want     int64
	}{{"new stream", false, 13}, {"existing stream", true, 14}} {
		stream := uuid.Must(uuid.NewV7())
		if c.existing {
			e.add(t, scope, stream, 1)
		}

		// A concurrent command commits right after this one loaded the fold.
		e.counter.Folds.OnNextLoad(func() { e.add(t, scope, stream, 10) })

		before := e.counter.Folds.Loads.Load()
		e.add(t, scope, stream, 3)

		if loads := e.counter.Folds.Loads.Load() - before; loads != 3 {
			t.Errorf("%s: %d loads, want 3 (ours, the concurrent command's, our retry)", c.name, loads)
		}

		if fold, _ := e.load(t, scope, stream); fold.State.Value != c.want {
			t.Errorf("%s: value = %d, want %d (both commands applied)", c.name, fold.State.Value, c.want)
		}
	}
}

func TestAVersionConflictIsReportedAfterTheLastAttempt(t *testing.T) {
	t.Parallel()
	e := setup(t, eventstore.WithMaxAttempts(3))

	attempts := 0
	err := e.store.InTx(t.Context(), eventstore.Scope{}, func(context.Context, *eventstore.Tx) error {
		attempts++

		return fmt.Errorf("save: %w", eventstore.ErrVersionConflict)
	})

	if !errors.Is(err, eventstore.ErrVersionConflict) {
		t.Errorf("err = %v, want ErrVersionConflict", err)
	}

	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestAStaleFoldIsRebuiltFromItsStreamOnLoad(t *testing.T) {
	t.Parallel()
	e := setup(t)
	scope := eventstore.Scope{HouseholdID: uuid.New()}
	stream := uuid.Must(uuid.NewV7())

	e.add(t, scope, stream, 2)
	e.add(t, scope, stream, 3)

	// A new fold schema version: the stored fold is stale.
	e.counter.FoldSchemaVersion = 2
	corruptFold(t, e.urls.Owner, stream)

	res := e.add(t, scope, stream, 4)

	if res.State.Value != 9 || res.State.Events != 3 || res.Version != 3 {
		t.Errorf("after a stale load: %+v at version %d, want value 9 from 3 events at version 3", res.State, res.Version)
	}

	if fold, _ := e.load(t, scope, stream); fold.SchemaVersion != 2 {
		t.Errorf("saved fold schema version = %d, want 2", fold.SchemaVersion)
	}
}

func TestRebuildRecomputesEveryFoldOfEveryHousehold(t *testing.T) {
	t.Parallel()
	e := setup(t)
	h1, h2 := eventstore.Scope{HouseholdID: uuid.New()}, eventstore.Scope{HouseholdID: uuid.New()}
	s1, s2, s3 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	e.add(t, h1, s1, 1)
	e.add(t, h1, s1, 2)
	e.add(t, h2, s2, 5)
	e.add(t, eventstore.Scope{}, s3, 7) // instance level

	e.counter.FoldSchemaVersion = 2

	for _, s := range []uuid.UUID{s1, s2, s3} {
		corruptFold(t, e.urls.Owner, s)
	}

	if err := eventstore.Rebuild(t.Context(), e.reader, e.store, e.counter.RebuildTarget()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	for _, c := range []struct {
		scope  eventstore.Scope
		stream uuid.UUID
		value  int64
	}{{h1, s1, 3}, {h2, s2, 5}, {eventstore.Scope{}, s3, 7}} {
		fold, found := e.load(t, c.scope, c.stream)
		if !found || fold.State.Value != c.value || fold.SchemaVersion != 2 {
			t.Errorf("stream %s: fold %+v (found %v), want value %d at schema version 2", c.stream, fold, found, c.value)
		}
	}
}

// corruptFold overwrites a stored fold behind the runtime's back, as an
// outdated fold schema would leave it.
func corruptFold(t *testing.T, ownerURL string, stream uuid.UUID) {
	t.Helper()

	conn, err := pgx.Connect(context.Background(), ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(context.Background(), `UPDATE test_counters SET value = -1, events = 0 WHERE id = $1`, stream); err != nil {
		t.Fatal(err)
	}
}
