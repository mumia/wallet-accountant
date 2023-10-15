package eventstoredb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"io"
	"strings"
)

var _ eventhorizon.EventStore = &EventStore{}

type EventStore struct {
	client        EventStorerer
	contentType   esdb.ContentType
	aggregateType eventhorizon.AggregateType
	batchSize     uint64
}

func (e EventStore) Save(ctx context.Context, events []eventhorizon.Event, originalVersion int) error {
	// If no event return no error
	if len(events) == 0 {
		return nil
	}

	esdbEvents := make([]esdb.EventData, len(events))
	for i, event := range events {
		marshalledData, err := json.Marshal(event.Data())
		if err != nil {
			return err
		}

		marshalledMetadata, err := json.Marshal(event.Metadata())
		if err != nil {
			return err
		}

		esdbEvents[i] = esdb.EventData{
			ContentType: e.contentType,
			EventType:   event.EventType().String(),
			Data:        marshalledData,
			Metadata:    marshalledMetadata,
		}
	}

	var streamOptions esdb.AppendToStreamOptions
	version := uint64(originalVersion)
	if version > 0 {
		streamOptions.ExpectedRevision = esdb.StreamRevision{Value: version - 1}
	} else {
		streamOptions.ExpectedRevision = esdb.NoStream{}
	}

	/*writeResult*/
	_, err := e.client.AppendToStream(
		context.Background(),
		e.fullStreamName(events[0].AggregateID()),
		streamOptions,
		esdbEvents...,
	)
	if err != nil {
		if err, ok := esdb.FromError(err); !ok {
			return err
		}

		return err
	}

	return nil
}

func (e EventStore) Load(ctx context.Context, uuid uuid.UUID) ([]eventhorizon.Event, error) {
	return e.LoadFrom(ctx, uuid, 1)
}

// LoadFrom although EventStoreDb uses a 0-based event version, version needs to be a 1-based integer
// because event horizon forces a 1-based version
func (e EventStore) LoadFrom(ctx context.Context, id uuid.UUID, version int) ([]eventhorizon.Event, error) {
	streamID := e.fullStreamName(id)

	version = version - 1

	var events []eventhorizon.Event
	for {
		from := esdb.StreamRevision{Value: uint64(version)}
		stream, err := e.client.ReadStream(ctx, streamID, esdb.ReadStreamOptions{From: from}, ^e.batchSize)
		if err != nil {
			if err, ok := esdb.FromError(err); !ok {
				if err.Code() == esdb.ErrorCodeResourceNotFound {
					if version == 1 {
						return []eventhorizon.Event{}, nil
					}

					return nil, fmt.Errorf("event stream not found. Stream: %s", streamID)
				}
			}

			return nil, err
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		eventsInStream, eventCountInStream, lastVersion, err := e.convertStreamToEventList(
			stream,
			e.aggregateType,
			id,
			version,
		)
		if err != nil {
			return nil, err
		}

		events = append(events, eventsInStream...)

		if uint64(eventCountInStream) < e.batchSize {
			break
		}

		version = lastVersion
	}

	return events, nil
}

func (e EventStore) Close() error {
	return e.client.Close()
}

func (e EventStore) fullStreamName(id uuid.UUID) string {
	return fmt.Sprintf("%s-%s", e.aggregateType, id)
}

func (e EventStore) convertStreamToEventList(
	stream *esdb.ReadStream,
	aggregateType eventhorizon.AggregateType,
	aggregateId uuid.UUID,
	version int,
) ([]eventhorizon.Event, int, int, error) {
	var events []eventhorizon.Event
	errorIsEOF := false

	eventsInStream := 0
	lastVersion := 0
	for !errorIsEOF {
		esdbEvent, err := stream.Recv()

		errorIsEOF = errors.Is(err, io.EOF)
		if errorIsEOF {
			continue
		}

		if err != nil {
			if err, ok := esdb.FromError(err); !ok {
				if err.Code() == esdb.ErrorCodeResourceNotFound && version == 0 {
					return []eventhorizon.Event{}, 0, 0, nil
				}
			}

			return nil, 0, 0, err
		}

		eventsInStream++

		if strings.HasPrefix(esdbEvent.OriginalEvent().EventType, "$") {
			continue
		}

		event, err := CreateEventForAggregate(esdbEvent, aggregateType, aggregateId)
		if err != nil {
			return nil, 0, 0, err
		}

		events = append(events, event)

		lastVersion = event.Version()
	}

	return events, eventsInStream, lastVersion, nil
}
