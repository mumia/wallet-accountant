package eventstore

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RebuildTarget is one aggregate's part in a rebuild: its stream type and
// how to rebuild one of its folds from the stream's events.
type RebuildTarget struct {
	StreamType string
	Rebuild    func(ctx context.Context, tx *Tx, streamID uuid.UUID, events []CloudEvent) error
}

// Rebuild rebuilds every fold of the targets from the event store, as the
// rebuild run mode does after a fold schema change (ADR-0011). It reads
// streams through reader (wa_event_store_reader, which sees every
// Household) and writes each fold through store as wa_app, scoped to the
// stream's Household (ADR-0014).
func Rebuild(ctx context.Context, reader Querier, store *Store, targets ...RebuildTarget) error {
	for _, target := range targets {
		streams, err := listStreams(ctx, reader, target.StreamType)
		if err != nil {
			return err
		}

		for _, s := range streams {
			err := store.InTx(ctx, Scope{HouseholdID: s.household}, func(ctx context.Context, tx *Tx) error {
				events, err := ReadStream(ctx, reader, s.id)
				if err != nil {
					return err
				}

				return target.Rebuild(ctx, tx, s.id, events)
			})
			if err != nil {
				return fmt.Errorf("rebuild %s %s: %w", target.StreamType, s.id, err)
			}
		}
	}

	return nil
}

type stream struct{ id, household uuid.UUID }

func listStreams(ctx context.Context, reader Querier, streamType string) ([]stream, error) {
	rows, err := reader.Query(ctx, `SELECT DISTINCT subject, householdid FROM event_store WHERE streamtype = $1`, streamType)
	if err != nil {
		return nil, fmt.Errorf("list %s streams: %w", streamType, err)
	}

	streams, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (stream, error) {
		var (
			s         stream
			household *uuid.UUID
		)

		if err := r.Scan(&s.id, &household); err != nil {
			return stream{}, fmt.Errorf("scan stream: %w", err)
		}

		s.household = derefID(household)

		return s, nil
	})
	if err != nil {
		return nil, fmt.Errorf("list %s streams: %w", streamType, err)
	}

	return streams, nil
}
