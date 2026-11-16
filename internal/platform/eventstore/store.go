package eventstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrVersionConflict means another transaction changed a stream after this
// one loaded it. Store.InTx retries the transaction when fn returns it.
var ErrVersionConflict = errors.New("stream version conflict")

const defaultMaxAttempts = 5

// Store runs commands in database transactions as wa_app, under row-level
// security (ADR-0014).
type Store struct {
	pool        *pgxpool.Pool
	now         func() time.Time
	maxAttempts int
}

// Option configures a Store.
type Option func(*Store)

// WithClock replaces time.Now for stamping events.
func WithClock(now func() time.Time) Option { return func(s *Store) { s.now = now } }

// WithMaxAttempts sets how often a transaction is tried before a version
// conflict is returned (default 5).
func WithMaxAttempts(n int) Option { return func(s *Store) { s.maxAttempts = n } }

// NewStore returns a Store using pool, which connects as wa_app.
func NewStore(pool *pgxpool.Pool, opts ...Option) *Store {
	s := &Store{pool: pool, now: time.Now, maxAttempts: defaultMaxAttempts}
	for _, o := range opts {
		o(s)
	}

	return s
}

// Scope is who and what a transaction acts for. A zero HouseholdID is
// instance scope (ADR-0017).
type Scope struct {
	HouseholdID   uuid.UUID
	ActorID       uuid.UUID
	CorrelationID uuid.UUID
	CausationID   uuid.UUID
}

// Tx is a database transaction of the Store. Repositories and fold stores
// query through its embedded pgx.Tx.
type Tx struct {
	pgx.Tx

	scope Scope
	now   func() time.Time
}

// HouseholdID returns the Household the transaction is scoped to, or
// uuid.Nil in instance scope.
func (t *Tx) HouseholdID() uuid.UUID { return t.scope.HouseholdID }

// SetHousehold switches the transaction to a Household, as a request does
// once it has resolved its User in instance scope.
func (t *Tx) SetHousehold(ctx context.Context, household uuid.UUID) error {
	if err := setHousehold(ctx, t.Tx, household); err != nil {
		return err
	}

	t.scope.HouseholdID = household

	return nil
}

// Now returns the current time on the Store's clock, the one that stamps
// the transaction's events, for a command that depends on the day it is
// given.
func (t *Tx) Now() time.Time { return t.now().UTC() }

// SetActor names the User the transaction's events are recorded for, as a
// request does once it has resolved its User.
func (t *Tx) SetActor(user uuid.UUID) { t.scope.ActorID = user }

// HandlerTx lets a deferred handler run commands in the transaction it was
// given for cause, which forwarding has already set to cause's Household.
// The events it records belong to that Household, are caused by cause, keep
// its correlation, and carry the trace of ctx. They have no actor: no User
// asked for them.
func HandlerTx(tx pgx.Tx, cause CloudEvent) *Tx {
	correlation := cause.CorrelationID
	if correlation == uuid.Nil {
		correlation = cause.ID
	}

	return &Tx{Tx: tx, now: time.Now, scope: Scope{
		HouseholdID:   cause.HouseholdID,
		CorrelationID: correlation,
		CausationID:   cause.ID,
	}}
}

func setHousehold(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
	setting := ""
	if household != uuid.Nil {
		setting = household.String()
	}

	if _, err := tx.Exec(ctx, `SELECT set_config('app.household_id', $1, true)`, setting); err != nil {
		return fmt.Errorf("set household: %w", err)
	}

	return nil
}

// InTx runs fn in one transaction scoped to scope and commits it. When fn
// (or the commit) reports a version conflict, the whole transaction is
// rolled back and fn runs again, up to the Store's maximum attempts.
func (s *Store) InTx(ctx context.Context, scope Scope, fn func(ctx context.Context, tx *Tx) error) error {
	var err error

	for range s.maxAttempts {
		err = s.attempt(ctx, scope, fn)
		if !errors.Is(err, ErrVersionConflict) {
			return err
		}
	}

	return err
}

func (s *Store) attempt(ctx context.Context, scope Scope, fn func(ctx context.Context, tx *Tx) error) error {
	pgtx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	defer pgtx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // a no-op after commit

	if err := setHousehold(ctx, pgtx, scope.HouseholdID); err != nil {
		return err
	}

	if err := fn(ctx, &Tx{Tx: pgtx, scope: scope, now: s.now}); err != nil {
		return asConflict(err)
	}

	if err := pgtx.Commit(ctx); err != nil {
		return asConflict(fmt.Errorf("commit: %w", err))
	}

	return nil
}

// asConflict maps a unique violation of a stream version to
// ErrVersionConflict, so it is retried and never leaked: the constraint spans
// Households that row-level security hides from each other.
func asConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "event_store_stream_version_key" {
		return fmt.Errorf("%w: %s", ErrVersionConflict, pgErr.Message)
	}

	return err
}
