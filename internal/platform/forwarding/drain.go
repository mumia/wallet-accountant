package forwarding

import (
	"context"
	"fmt"
	"time"

	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// drainPoll is how often Drain checks the consumer groups' offsets.
const drainPoll = 10 * time.Millisecond

// Drain waits until every consumer group in groups has handled every event
// committed when Drain was called, so tests can assert deferred effects
// without sleeping. A forwarder must be running the groups. reader connects
// as wa_event_store_reader.
func Drain(ctx context.Context, reader eventstore.Querier, groups ...string) error {
	if len(groups) == 0 {
		return nil
	}

	// The last committed event, in delivery order. Events of transactions
	// still running are not committed yet, so they are not pending. Order by
	// the xid8 column, not the text output column: as text "1003" < "992".
	rows, err := reader.Query(ctx, `
		SELECT e.transaction_id::text AS txid, e.global_position FROM event_store e
		ORDER BY e.transaction_id DESC, e.global_position DESC LIMIT 1`)
	if err != nil {
		return fmt.Errorf("drain: last event: %w", err)
	}

	var (
		txid     string
		position int64
		found    bool
	)

	for rows.Next() {
		found = true

		if err := rows.Scan(&txid, &position); err != nil {
			rows.Close()

			return fmt.Errorf("drain: last event: %w", err)
		}
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return fmt.Errorf("drain: last event: %w", err)
	}

	if !found {
		return nil
	}

	for {
		pending, err := pendingGroups(ctx, reader, groups, txid, position)
		if err != nil {
			return err
		}

		if pending == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("drain: %d consumer groups still behind: %w", pending, ctx.Err())
		case <-time.After(drainPoll):
		}
	}
}

func pendingGroups(ctx context.Context, reader eventstore.Querier, groups []string, txid string, position int64) (int, error) {
	rows, err := reader.Query(ctx, `
		SELECT count(*) FROM unnest($1::text[]) AS g(name)
		WHERE NOT EXISTS (
			SELECT FROM `+offsetsTable+` o
			WHERE o.consumer_group = g.name
			  AND (o.last_processed_transaction_id, o.offset_acked) >= ($2::text::xid8, $3::bigint)
		)`, groups, txid, position)
	if err != nil {
		return 0, fmt.Errorf("drain: offsets: %w", err)
	}
	defer rows.Close()

	var pending int

	for rows.Next() {
		if err := rows.Scan(&pending); err != nil {
			return 0, fmt.Errorf("drain: offsets: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("drain: offsets: %w", err)
	}

	return pending, nil
}
