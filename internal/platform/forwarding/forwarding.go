// Package forwarding forwards events from the event store to deferred
// handlers in-process, through Watermill (ADR-0011, ADR-0013). Each handler
// is its own consumer group. Delivery is at least once and the effect
// exactly once: every attempt runs inside a SAVEPOINT of the subscriber's
// transaction, as wa_app scoped to the event's Household (ADR-0014), and
// records the event as processed in the same transaction.
package forwarding

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// Handler is a deferred effect of events. Handle runs in a transaction as
// wa_app, scoped to the event's Household (instance scope for an
// instance-level event). Returning an error rolls the attempt back and the
// event is delivered again.
type Handler struct {
	// Name is the handler's consumer group. Renaming it replays every event.
	Name   string
	Handle func(ctx context.Context, tx pgx.Tx, event eventstore.CloudEvent) error
}

// Config configures the forwarder.
type Config struct {
	// Reader connects as wa_event_store_reader.
	Reader   *pgxpool.Pool
	Handlers []Handler
	// Logger defaults to slog.Default().
	Logger *slog.Logger
	// TracerProvider defaults to a no-op provider.
	TracerProvider trace.TracerProvider
	// PollInterval is how long to wait when no event is pending (default 1s).
	PollInterval time.Duration
	// ResendInterval is how long to wait before retrying a failed event
	// (default 1s).
	ResendInterval time.Duration
	// AckDeadline is how long an attempt may run before Watermill gives up
	// on it and delivers the event again (default 30s).
	AckDeadline time.Duration
	// CloseTimeout bounds waiting for running handlers on shutdown
	// (default 30s).
	CloseTimeout time.Duration
}

// Run forwards events to the handlers until ctx is cancelled.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.TracerProvider == nil {
		cfg.TracerProvider = noop.NewTracerProvider()
	}

	logger := watermill.NewSlogLogger(cfg.Logger)

	router, err := message.NewRouter(message.RouterConfig{CloseTimeout: cfg.CloseTimeout}, logger)
	if err != nil {
		return fmt.Errorf("forwarding: router: %w", err)
	}

	tracer := cfg.TracerProvider.Tracer("github.com/mumia/wallet-accountant/internal/platform/forwarding")

	for _, h := range cfg.Handlers {
		sub, err := wsql.NewSubscriber(guardedBeginner{wsql.BeginnerFromPgx(cfg.Reader)}, wsql.SubscriberConfig{
			ConsumerGroup:  h.Name,
			SchemaAdapter:  eventStoreSchema{},
			OffsetsAdapter: offsets,
			PollInterval:   cfg.PollInterval,
			ResendInterval: cfg.ResendInterval,
			AckDeadline:    ackDeadline(cfg.AckDeadline),
		}, logger)
		if err != nil {
			return fmt.Errorf("forwarding: subscriber %s: %w", h.Name, err)
		}

		router.AddConsumerHandler(h.Name, topic, sub, middleware(h, tracer, cfg.Logger))
	}

	runErr := make(chan error, 1)

	go func() { runErr <- router.Run(ctx) }()

	select {
	case err := <-runErr:
		// Subscribing fails when the forwarder is stopped while starting.
		if err != nil && ctx.Err() == nil {
			return fmt.Errorf("forwarding: %w", err)
		}

		return nil
	case <-ctx.Done():
	}

	// Stopped while subscribing, the router never runs: subscribing fails on
	// the cancelled context, and closing it would wait for a handler that
	// never started.
	select {
	case <-router.Running():
	case <-runErr: //nolint:errcheck // the subscription failed because ctx was cancelled
		return nil
	}

	closeErr := router.Close()

	if err := <-runErr; err != nil {
		return fmt.Errorf("forwarding: %w", err)
	}

	if closeErr != nil {
		return fmt.Errorf("forwarding: close: %w", closeErr)
	}

	return nil
}

var errNoTransaction = errors.New("message carries no subscriber transaction")

// middleware runs one delivery attempt of h: SAVEPOINT, SET LOCAL ROLE
// wa_app, set app.household_id from the event, skip events already
// processed, handle, then RELEASE and RESET ROLE before the ack (the ack
// query runs as the reader). A failed attempt rolls back to the savepoint
// before the Nack, because Watermill commits its transaction even when a
// message is not acked.
func middleware(h Handler, tracer trace.Tracer, logger *slog.Logger) message.NoPublishHandlerFunc {
	return func(msg *message.Message) error {
		var ev eventstore.CloudEvent
		if err := json.Unmarshal(msg.Payload, &ev); err != nil {
			return fmt.Errorf("forwarding: %s: %w", h.Name, err)
		}

		ctx := propagation.TraceContext{}.Extract(msg.Context(), propagation.MapCarrier{
			"traceparent": ev.TraceParent,
			"tracestate":  ev.TraceState,
		})

		ctx, span := tracer.Start(ctx, "handle "+ev.Type, trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.consumer.group.name", h.Name),
				attribute.String("messaging.message.id", ev.ID.String()),
			))
		defer span.End()

		wtx, ok := wsql.TxFromContext(msg.Context())
		if !ok {
			return errNoTransaction
		}

		guarded, ok := wtx.(*guardedTx)
		if !ok {
			return fmt.Errorf("%w: not a guarded pgx transaction", errNoTransaction)
		}

		// The subscriber can't end the transaction while the attempt uses it.
		guarded.mu.Lock()
		defer guarded.mu.Unlock()

		tx := guarded.pgx

		if _, err := tx.Exec(ctx, `SAVEPOINT handler_attempt`); err != nil {
			return fail(ctx, span, logger, h, ev, fmt.Errorf("savepoint: %w", err))
		}

		if err := attempt(ctx, tx, h, ev); err != nil {
			if ctx.Err() == nil {
				// Also undoes SET LOCAL ROLE and set_config.
				if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT handler_attempt`); rbErr != nil {
					err = errors.Join(err, fmt.Errorf("rollback to savepoint: %w", rbErr))
				}
			}

			return fail(ctx, span, logger, h, ev, err)
		}

		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT handler_attempt; RESET ROLE`); err != nil {
			return fail(ctx, span, logger, h, ev, fmt.Errorf("release savepoint: %w", err))
		}

		return nil
	}
}

// guardedBeginner begins subscriber transactions that one attempt at a time
// can hold. Watermill hands its transaction to the handler's goroutine, but
// stops waiting for the handler on shutdown or at the ack deadline and then
// commits or rolls back at once, while the handler may still be running
// queries: a pgx connection used by two goroutines, which corrupts it. Every
// use by the subscriber (including Commit and Rollback) waits for the
// running attempt to return, so its writes are committed or rolled back
// whole, as ADR-0011 expects.
type guardedBeginner struct {
	wsql.Beginner
}

func (b guardedBeginner) BeginTx(ctx context.Context, opts *sql.TxOptions) (wsql.Tx, error) {
	tx, err := b.Beginner.BeginTx(ctx, opts)
	if err != nil {
		return nil, err //nolint:wrapcheck // Watermill wraps it
	}

	pgxTx, ok := tx.(*wsql.PgxTx)
	if !ok {
		return nil, fmt.Errorf("forwarding: %T is not a pgx transaction", tx)
	}

	return &guardedTx{tx: tx, pgx: pgxTx.Tx}, nil
}

type guardedTx struct {
	mu  sync.Mutex
	tx  wsql.Tx
	pgx pgx.Tx
}

func (t *guardedTx) ExecContext(ctx context.Context, query string, args ...any) (wsql.Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.tx.ExecContext(ctx, query, args...) //nolint:wrapcheck // Watermill wraps it
}

func (t *guardedTx) QueryContext(ctx context.Context, query string, args ...any) (wsql.Rows, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.tx.QueryContext(ctx, query, args...) //nolint:wrapcheck // Watermill wraps it
}

func (t *guardedTx) Commit() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.tx.Commit() //nolint:wrapcheck // Watermill wraps it
}

func (t *guardedTx) Rollback() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.tx.Rollback() //nolint:wrapcheck // Watermill wraps it
}

func attempt(ctx context.Context, tx pgx.Tx, h Handler, ev eventstore.CloudEvent) error {
	household := ""
	if ev.HouseholdID != uuid.Nil {
		household = ev.HouseholdID.String()
	}

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE wa_app`); err != nil {
		return fmt.Errorf("set role: %w", err)
	}

	if _, err := tx.Exec(ctx, `SELECT set_config('app.household_id', $1, true)`, household); err != nil {
		return fmt.Errorf("set household: %w", err)
	}

	tag, err := tx.Exec(ctx, `INSERT INTO processed_events (consumer_group, event_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, h.Name, ev.ID)
	if err != nil {
		return fmt.Errorf("record processed event: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return nil // processed before a shutdown cut the ack short
	}

	return h.Handle(ctx, tx, ev)
}

func fail(ctx context.Context, span trace.Span, logger *slog.Logger, h Handler, ev eventstore.CloudEvent, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	logger.ErrorContext(ctx, "event handler failed",
		"consumer_group", h.Name, "event_id", ev.ID.String(), "event_type", ev.Type, "error", err.Error())

	return err
}

func ackDeadline(d time.Duration) *time.Duration {
	if d == 0 {
		return nil
	}

	return &d
}
