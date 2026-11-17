package forwarding

import (
	stdsql "database/sql"
	"encoding/json"
	"errors"
	"fmt"

	wsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// offsetsTable is Watermill's offsets table, created by our migrations.
const offsetsTable = "watermill_offsets_events"

// topic is the only topic: the event store.
const topic = "events"

var offsets = wsql.DefaultPostgreSQLOffsetsAdapter{
	GenerateMessagesOffsetsTableName: func(string) string { return offsetsTable },
}

// eventStoreSchema is Watermill's view of the event store (ADR-0011). It
// only selects and unmarshals: the event store repository inserts, and the
// migrations own the tables.
type eventStoreSchema struct{}

var errInsertNotSupported = errors.New("events are appended by the event store, never by Watermill")

func (eventStoreSchema) InsertQuery(wsql.InsertQueryParams) (wsql.Query, error) {
	return wsql.Query{}, errInsertNotSupported
}

// SelectQuery returns the next events after the consumer group's offset, in
// (transaction_id, global_position) order. Gap safety: only rows of
// transactions older than every transaction still running are visible, so an
// event committed after a slower transaction waits for it.
func (eventStoreSchema) SelectQuery(p wsql.SelectQueryParams) (wsql.Query, error) {
	next, err := p.OffsetsAdapter.NextOffsetQuery(wsql.NextOffsetQueryParams{Topic: p.Topic, ConsumerGroup: p.ConsumerGroup})
	if err != nil {
		return wsql.Query{}, fmt.Errorf("next offset query: %w", err)
	}

	q := `
	SELECT * FROM (
		WITH last_processed AS (` + next.Query + `)
		SELECT ` + eventstore.Columns + `, global_position, transaction_id
		FROM event_store
		WHERE (
				(transaction_id = (SELECT last_processed_transaction_id FROM last_processed)
				 AND global_position > (SELECT offset_acked FROM last_processed))
			 OR transaction_id > (SELECT last_processed_transaction_id FROM last_processed)
			  )
		  AND transaction_id < pg_snapshot_xmin(pg_current_snapshot())
	) AS events
	ORDER BY transaction_id, global_position
	LIMIT 100`

	return wsql.Query{Query: q, Args: next.Args}, nil
}

// UnmarshalMessage turns a row into a Watermill message whose payload is the
// event as a CloudEvent in structured JSON mode.
func (eventStoreSchema) UnmarshalMessage(p wsql.UnmarshalMessageParams) (wsql.Row, error) {
	var (
		row  wsql.Row
		txid wsql.XID8
	)

	ev, err := eventstore.ScanEvent(p.Row, &row.Offset, &txid)
	if err != nil {
		return wsql.Row{}, err //nolint:wrapcheck // already wrapped
	}

	payload, err := json.Marshal(ev)
	if err != nil {
		return wsql.Row{}, fmt.Errorf("encode event %s: %w", ev.ID, err)
	}

	row.UUID = []byte(ev.ID.String())
	row.Payload = payload
	row.Msg = message.NewMessage(ev.ID.String(), payload)
	row.Msg.Metadata.Set("ce_type", ev.Type)
	// The offsets adapter's ack query reads it.
	row.ExtraData = map[string]any{"transaction_id": txid}

	return row, nil
}

func (eventStoreSchema) SchemaInitializingQueries(wsql.SchemaInitializingQueriesParams) ([]wsql.Query, error) {
	return nil, nil
}

func (eventStoreSchema) SubscribeIsolationLevel() stdsql.IsolationLevel {
	return stdsql.LevelRepeatableRead
}
