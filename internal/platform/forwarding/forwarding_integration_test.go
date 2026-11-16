//go:build integration

package forwarding_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore/eventstoretest"
	"github.com/mumia/wallet-accountant/internal/platform/forwarding"
)

type env struct {
	urls    databasetest.URLs
	reader  *pgxpool.Pool
	owner   *pgxpool.Pool
	store   *eventstore.Store
	counter *eventstoretest.Counter
	spans   *tracetest.InMemoryExporter
}

func setup(t *testing.T) *env {
	t.Helper()

	urls := databasetest.New(t)
	e := &env{
		urls:    urls,
		reader:  pool(t, urls.Reader),
		owner:   pool(t, urls.Owner),
		store:   eventstore.NewStore(pool(t, urls.App)),
		counter: eventstoretest.New(t, urls.Owner),
		spans:   tracetest.NewInMemoryExporter(),
	}

	// Effects handlers record, Household-scoped like any projection.
	_, err := e.owner.Exec(t.Context(), `
		CREATE TABLE test_effects (
			consumer_group text NOT NULL,
			event_id       uuid NOT NULL,
			household_id   uuid
		);
		ALTER TABLE test_effects ENABLE ROW LEVEL SECURITY;
		ALTER TABLE test_effects FORCE ROW LEVEL SECURITY;
		CREATE POLICY household_or_instance ON test_effects
			USING ((household_id IS NULL AND NULLIF(current_setting('app.household_id', true), '') IS NULL)
				OR household_id = NULLIF(current_setting('app.household_id', true), '')::uuid)
			WITH CHECK ((household_id IS NULL AND NULLIF(current_setting('app.household_id', true), '') IS NULL)
				OR household_id = NULLIF(current_setting('app.household_id', true), '')::uuid);
		GRANT SELECT, INSERT ON test_effects TO wa_app;`)
	if err != nil {
		t.Fatal(err)
	}

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

// add records one counter event and returns its ID.
func (e *env) add(t *testing.T, ctx context.Context, household uuid.UUID) uuid.UUID {
	t.Helper()

	var id uuid.UUID

	err := e.store.InTx(ctx, eventstore.Scope{HouseholdID: household}, func(ctx context.Context, tx *eventstore.Tx) error {
		res, err := e.counter.Execute(ctx, tx, uuid.Must(uuid.NewV7()), eventstoretest.Add{N: 1})
		if err == nil {
			id = res.Events[0].ID
		}

		return err
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	return id
}

// start runs a forwarder with handlers until the test ends or stop is called.
func (e *env) start(t *testing.T, handlers ...forwarding.Handler) (stop func()) {
	t.Helper()

	return e.startWith(t, forwarding.Config{Handlers: handlers})
}

func (e *env) startWith(t *testing.T, cfg forwarding.Config) (stop func()) {
	t.Helper()

	cfg.Reader = e.reader
	cfg.TracerProvider = sdktrace.NewTracerProvider(sdktrace.WithSyncer(e.spans))
	cfg.PollInterval = 10 * time.Millisecond
	cfg.ResendInterval = 10 * time.Millisecond
	cfg.CloseTimeout = 5 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- forwarding.Run(ctx, cfg)
	}()

	var once sync.Once

	stop = func() {
		once.Do(func() {
			cancel()

			if err := <-done; err != nil {
				t.Errorf("forwarder stopped with an error: %v", err)
			}
		})
	}
	t.Cleanup(stop)

	return stop
}

func (e *env) drain(t *testing.T, groups ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	if err := forwarding.Drain(ctx, e.reader, groups...); err != nil {
		t.Fatalf("drain: %v", err)
	}
}

// effects returns how often each event's effect was committed for group.
func (e *env) effects(t *testing.T, group string) map[uuid.UUID]int {
	t.Helper()

	rows, err := e.owner.Query(t.Context(), `SELECT event_id, count(*) FROM test_effects WHERE consumer_group = $1 GROUP BY event_id`, group)
	if err != nil {
		t.Fatal(err)
	}

	counts := map[uuid.UUID]int{}

	for rows.Next() {
		var (
			id uuid.UUID
			n  int
		)

		if err := rows.Scan(&id, &n); err != nil {
			t.Fatal(err)
		}

		counts[id] = n
	}

	return counts
}

func writeEffect(ctx context.Context, tx pgx.Tx, group string, ev eventstore.CloudEvent) error {
	var household *uuid.UUID
	if ev.HouseholdID != uuid.Nil {
		household = &ev.HouseholdID
	}

	_, err := tx.Exec(ctx, `INSERT INTO test_effects (consumer_group, event_id, household_id) VALUES ($1, $2, $3)`, group, ev.ID, household)

	return err
}

// recorder is a handler that records each delivery and writes one effect.
type recorder struct {
	group string

	mu   sync.Mutex
	seen []uuid.UUID
}

func (r *recorder) handler() forwarding.Handler {
	return forwarding.Handler{Name: r.group, Handle: func(ctx context.Context, tx pgx.Tx, ev eventstore.CloudEvent) error {
		r.mu.Lock()
		r.seen = append(r.seen, ev.ID)
		r.mu.Unlock()

		return writeEffect(ctx, tx, r.group, ev)
	}}
}

func (r *recorder) delivered() []uuid.UUID {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.seen)
}

func TestEventsAreForwardedExactlyOnceInOrder(t *testing.T) {
	t.Parallel()
	e := setup(t)
	rec := &recorder{group: "projection"}
	e.start(t, rec.handler())

	h := uuid.New()
	want := []uuid.UUID{e.add(t, t.Context(), h), e.add(t, t.Context(), h), e.add(t, t.Context(), uuid.Nil)}

	e.drain(t, rec.group)

	if got := rec.delivered(); !slices.Equal(got, want) {
		t.Errorf("delivered %v, want %v", got, want)
	}

	for _, id := range want {
		if n := e.effects(t, rec.group)[id]; n != 1 {
			t.Errorf("event %s: %d effects, want 1", id, n)
		}
	}
}

// Transaction IDs order numerically, not as text: Drain once compared them
// as text, so "1003" < "992" and it returned before the newer events were
// handled whenever the transaction ID gained a digit.
func TestDrainWaitsForEventsWhoseTransactionIDGainedADigit(t *testing.T) {
	t.Parallel()
	e := setup(t)
	rec := &recorder{group: "projection"}
	e.start(t, rec.handler())

	before := e.add(t, t.Context(), uuid.Nil)
	e.drain(t, rec.group)

	var txid uint64
	if err := e.owner.QueryRow(t.Context(), `SELECT transaction_id::text::bigint FROM event_store WHERE id = $1`, before).Scan(&txid); err != nil {
		t.Fatal(err)
	}

	// Use up transaction IDs until the next one has one more digit.
	next := uint64(10)
	for next <= txid {
		next *= 10
	}

	_, err := e.owner.Exec(t.Context(), fmt.Sprintf(`
		DO $$
		BEGIN
			WHILE pg_current_xact_id()::text::bigint < %d LOOP
				COMMIT;
			END LOOP;
		END $$`, next), pgx.QueryExecModeSimpleProtocol)
	if err != nil {
		t.Fatal(err)
	}

	after := e.add(t, t.Context(), uuid.Nil)
	e.drain(t, rec.group)

	if n := e.effects(t, rec.group)[after]; n != 1 {
		t.Errorf("event of a transaction past %d: %d effects after Drain, want 1", next, n)
	}
}

func TestAnEventCommittedAfterASlowerTransactionWaitsForIt(t *testing.T) {
	t.Parallel()
	e := setup(t)
	rec := &recorder{group: "projection"}
	e.start(t, rec.handler())

	h := uuid.New()
	appended, commit := make(chan uuid.UUID), make(chan struct{})
	slow := make(chan error, 1)

	go func() {
		slow <- e.store.InTx(context.Background(), eventstore.Scope{HouseholdID: h}, func(ctx context.Context, tx *eventstore.Tx) error {
			res, err := e.counter.Execute(ctx, tx, uuid.Must(uuid.NewV7()), eventstoretest.Add{N: 1})
			if err != nil {
				close(appended)

				return err
			}

			appended <- res.Events[0].ID
			<-commit

			return nil
		})
	}()

	late, ok := <-appended
	if !ok {
		t.Fatalf("slow transaction: %v", <-slow)
	}

	fast := e.add(t, t.Context(), h)

	// The fast event has the higher position but must wait.
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(rec.delivered()) > 0 {
			t.Fatalf("delivered %v while an earlier transaction was still open", rec.delivered())
		}

		time.Sleep(10 * time.Millisecond)
	}

	close(commit)

	if err := <-slow; err != nil {
		t.Fatal(err)
	}

	e.drain(t, rec.group)

	if got, want := rec.delivered(), []uuid.UUID{late, fast}; !slices.Equal(got, want) {
		t.Errorf("delivered %v, want the slow transaction's event first: %v", got, want)
	}
}

func TestANewConsumerGroupReplaysFromTheStart(t *testing.T) {
	t.Parallel()
	e := setup(t)
	first := &recorder{group: "first"}
	stop := e.start(t, first.handler())

	h := uuid.New()
	want := []uuid.UUID{e.add(t, t.Context(), h), e.add(t, t.Context(), h)}
	e.drain(t, first.group)
	stop()

	replay := &recorder{group: "replay"}
	e.start(t, replay.handler())
	e.drain(t, replay.group)

	if got := replay.delivered(); !slices.Equal(got, want) {
		t.Errorf("new consumer group got %v, want every event %v", got, want)
	}
}

func TestANackedAttemptLeavesNoEffect(t *testing.T) {
	t.Parallel()
	e := setup(t)

	var (
		mu       sync.Mutex
		attempts = map[uuid.UUID]int{}
	)

	e.start(t, forwarding.Handler{Name: "flaky", Handle: func(ctx context.Context, tx pgx.Tx, ev eventstore.CloudEvent) error {
		if err := writeEffect(ctx, tx, "flaky", ev); err != nil {
			return err
		}

		mu.Lock()
		defer mu.Unlock()

		attempts[ev.ID]++
		if attempts[ev.ID] == 1 {
			return errors.New("first attempt fails after writing")
		}

		return nil
	}})

	id := e.add(t, t.Context(), uuid.New())
	e.drain(t, "flaky")

	mu.Lock()
	defer mu.Unlock()

	if attempts[id] != 2 {
		t.Errorf("attempts = %d, want 2", attempts[id])
	}

	if n := e.effects(t, "flaky")[id]; n != 1 {
		t.Errorf("effects = %d, want 1: the failed attempt's write must be rolled back", n)
	}
}

func TestAnEventRedeliveredAfterAShutdownChangesNothing(t *testing.T) {
	t.Parallel()
	e := setup(t)

	entered := make(chan struct{})

	var (
		mu    sync.Mutex
		calls int
	)

	// The first run writes its effect, then is shut down before it acks.
	stop := e.start(t, forwarding.Handler{Name: "crash", Handle: func(ctx context.Context, tx pgx.Tx, ev eventstore.CloudEvent) error {
		if err := writeEffect(ctx, tx, "crash", ev); err != nil {
			return err
		}

		mu.Lock()
		calls++
		mu.Unlock()

		close(entered)
		<-ctx.Done()

		return ctx.Err()
	}})

	id := e.add(t, t.Context(), uuid.New())

	<-entered
	stop()

	rec := &recorder{group: "crash"}
	e.start(t, rec.handler())
	e.add(t, t.Context(), uuid.New()) // something after it, so the drain has a target past it
	e.drain(t, "crash")

	if n := e.effects(t, "crash")[id]; n != 1 {
		t.Errorf("effects = %d, want exactly 1 across the shutdown", n)
	}

	mu.Lock()
	defer mu.Unlock()

	if calls != 1 {
		t.Errorf("first run called %d times, want 1", calls)
	}
}

func TestAnEventRedeliveredAfterItsWritesWereCommittedUnackedChangesNothing(t *testing.T) {
	t.Parallel()
	e := setup(t)

	var (
		mu    sync.Mutex
		calls = map[uuid.UUID]int{}
	)

	// The first attempt outlives its ack deadline: Watermill gives up on it
	// and commits its transaction, writes included, without moving the offset.
	e.startWith(t, forwarding.Config{AckDeadline: 200 * time.Millisecond, Handlers: []forwarding.Handler{{
		Name: "slow",
		Handle: func(ctx context.Context, tx pgx.Tx, ev eventstore.CloudEvent) error {
			mu.Lock()
			calls[ev.ID]++
			first := calls[ev.ID] == 1
			mu.Unlock()

			if err := writeEffect(ctx, tx, "slow", ev); err != nil {
				return err
			}

			if first {
				<-ctx.Done()

				return ctx.Err()
			}

			return nil
		},
	}}})

	id := e.add(t, t.Context(), uuid.New())
	e.drain(t, "slow")

	if n := e.effects(t, "slow")[id]; n != 1 {
		t.Errorf("effects = %d, want exactly 1 although the event was delivered again", n)
	}

	mu.Lock()
	defer mu.Unlock()

	if calls[id] != 1 {
		t.Errorf("handler called %d times, want 1: the redelivery must be skipped as processed", calls[id])
	}
}

func TestAHandlerActsOnlyWithinTheEventsHousehold(t *testing.T) {
	t.Parallel()
	e := setup(t)
	h1, h2 := uuid.New(), uuid.New()

	type seen struct {
		user       string
		households int
		crossErr   error
	}

	var (
		mu  sync.Mutex
		obs = map[uuid.UUID]seen{}
	)

	e.start(t, forwarding.Handler{Name: "nosy", Handle: func(ctx context.Context, tx pgx.Tx, ev eventstore.CloudEvent) error {
		var s seen

		if err := tx.QueryRow(ctx, `SELECT current_user, (SELECT count(DISTINCT coalesce(household_id::text, 'instance')) FROM test_effects)`).
			Scan(&s.user, &s.households); err != nil {
			return err
		}

		// Try to write into another Household.
		other := h2
		if ev.HouseholdID == h2 {
			other = h1
		}

		if _, err := tx.Exec(ctx, `SAVEPOINT probe`); err != nil {
			return err
		}

		_, s.crossErr = tx.Exec(ctx, `INSERT INTO test_effects (consumer_group, event_id, household_id) VALUES ('nosy', $1, $2)`, ev.ID, other)

		if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT probe`); err != nil {
			return err
		}

		mu.Lock()
		obs[ev.ID] = s
		mu.Unlock()

		return writeEffect(ctx, tx, "nosy", ev)
	}})

	// Effects in both Households and in instance scope already exist.
	e.add(t, t.Context(), h1)
	e.add(t, t.Context(), h2)
	e.add(t, t.Context(), uuid.Nil)
	e.drain(t, "nosy")

	inH1 := e.add(t, t.Context(), h1)
	instance := e.add(t, t.Context(), uuid.Nil)
	e.drain(t, "nosy")

	mu.Lock()
	defer mu.Unlock()

	for name, id := range map[string]uuid.UUID{"Household event": inH1, "instance-level event": instance} {
		s := obs[id]
		if s.user != "wa_app" {
			t.Errorf("%s: handler ran as %q, want wa_app", name, s.user)
		}

		if s.households != 1 {
			t.Errorf("%s: handler saw effects of %d scopes, want only its own", name, s.households)
		}

		if s.crossErr == nil || !strings.Contains(s.crossErr.Error(), "row-level security") {
			t.Errorf("%s: writing into another Household: err = %v, want a row-level security violation", name, s.crossErr)
		}
	}
}

func TestAHandlerContinuesTheTraceOfTheCommand(t *testing.T) {
	t.Parallel()
	e := setup(t)
	rec := &recorder{group: "traced"}
	e.start(t, rec.handler())

	ctx, span := sdktrace.NewTracerProvider().Tracer("test").Start(context.Background(), "request")
	e.add(t, ctx, uuid.New())
	span.End()

	e.drain(t, rec.group)

	for _, s := range e.spans.GetSpans() {
		if s.SpanContext.TraceID() == span.SpanContext().TraceID() && s.Parent.SpanID() == span.SpanContext().SpanID() {
			return
		}
	}

	t.Errorf("no handler span in trace %s; spans: %s", span.SpanContext().TraceID(), spanNames(e.spans.GetSpans()))
}

func spanNames(spans tracetest.SpanStubs) string {
	var names []string
	for _, s := range spans {
		names = append(names, fmt.Sprintf("%s(%s)", s.Name, s.SpanContext.TraceID()))
	}

	return strings.Join(names, ", ")
}
