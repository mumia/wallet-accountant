package eventstore

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier runs queries: a pool, a connection or a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Columns are the event store's columns in the order ScanEvent reads them.
const Columns = `id, source, type, subject, time, datacontenttype, dataschema, householdid, streamtype,
	streamversion, actorid, correlationid, causationid, traceparent, tracestate, data`

// appendEvents inserts recorded events. A stream version taken by another
// transaction fails with a unique violation, which InTx maps to a conflict.
func appendEvents(ctx context.Context, tx pgx.Tx, events []CloudEvent) error {
	for _, e := range events {
		_, err := tx.Exec(ctx, `INSERT INTO event_store (`+Columns+`)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			e.ID, e.Source, e.Type, e.Subject, e.Time, e.DataContentType, nullText(e.DataSchema),
			nullID(e.HouseholdID), e.StreamType, e.StreamVersion, nullID(e.ActorID), nullID(e.CorrelationID),
			nullID(e.CausationID), nullText(e.TraceParent), nullText(e.TraceState), []byte(e.Data))
		if err != nil {
			return fmt.Errorf("append %s to stream %s: %w", e.Type, e.Subject, err)
		}
	}

	return nil
}

// ReadStream returns a stream's events in version order. Through wa_app it
// sees only streams of the transaction's scope; through
// wa_event_store_reader, every stream.
func ReadStream(ctx context.Context, q Querier, streamID uuid.UUID) ([]CloudEvent, error) {
	rows, err := q.Query(ctx, `SELECT `+Columns+` FROM event_store WHERE subject = $1 ORDER BY streamversion`, streamID)
	if err != nil {
		return nil, fmt.Errorf("read stream %s: %w", streamID, err)
	}

	events, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (CloudEvent, error) { return ScanEvent(r) })
	if err != nil {
		return nil, fmt.Errorf("read stream %s: %w", streamID, err)
	}

	return events, nil
}

// ScanEvent reads a row selected as Columns, optionally followed by more
// columns passed as extra.
func ScanEvent(row pgx.Row, extra ...any) (CloudEvent, error) {
	var (
		e                                  CloudEvent
		schema, traceParent, traceState    *string
		household, actor, correl, causedBy *uuid.UUID
		data                               []byte
	)

	dest := append([]any{
		&e.ID, &e.Source, &e.Type, &e.Subject, &e.Time, &e.DataContentType, &schema, &household,
		&e.StreamType, &e.StreamVersion, &actor, &correl, &causedBy, &traceParent, &traceState, &data,
	}, extra...)

	if err := row.Scan(dest...); err != nil {
		return CloudEvent{}, fmt.Errorf("scan event: %w", err)
	}

	e.DataSchema, e.TraceParent, e.TraceState = deref(schema), deref(traceParent), deref(traceState)
	e.HouseholdID, e.ActorID, e.CorrelationID, e.CausationID = derefID(household), derefID(actor), derefID(correl), derefID(causedBy)
	e.Data = data

	return e, nil
}

func nullID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}

	return &id
}

func nullText(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func derefID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}

	return *id
}
