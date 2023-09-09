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
	"walletaccountant/definitions"
)

var _ eventhorizon.EventStore = &EventStore{}

type EventStore struct {
	client        *esdb.Client
	contentType   esdb.ContentType
	aggregateType eventhorizon.AggregateType
}

type EventStoreFactory func(aggregateType eventhorizon.AggregateType) *EventStore

func NewEventStoreFactory(
	eventDataRegisters []definitions.EventDataRegisters,
	client *esdb.Client,
) (EventStoreFactory, error) {
	for _, eventDataRegister := range eventDataRegisters {
		for _, register := range eventDataRegister.Registers() {
			eventhorizon.RegisterEventData(register.EventType, register.EventData)
		}
	}

	return func(aggregateType eventhorizon.AggregateType) *EventStore {
		return &EventStore{
			client:        client,
			contentType:   esdb.ContentTypeJson,
			aggregateType: aggregateType,
		}
	}, nil
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
	if version > 1 {
		streamOptions.ExpectedRevision = esdb.StreamRevision{Value: version}
	} else if version == 1 {
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

func (e EventStore) LoadFrom(ctx context.Context, id uuid.UUID, version int) ([]eventhorizon.Event, error) {
	streamID := e.fullStreamName(id)

	from := esdb.StreamRevision{Value: uint64(version)}
	stream, err := e.client.ReadStream(ctx, streamID, esdb.ReadStreamOptions{From: from}, ^uint64(0))
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

	return e.convertStreamToEventList(stream, version)
}

func (e EventStore) Close() error {
	return e.client.Close()
}

func (e EventStore) fullStreamName(id uuid.UUID) string {
	return fmt.Sprintf("%s-%s", e.aggregateType, id)
}

func (e EventStore) convertStreamToEventList(stream *esdb.ReadStream, version int) ([]eventhorizon.Event, error) {
	var events []eventhorizon.Event
	errorIsEOF := false
	for !errorIsEOF {
		esdbEvent, err := stream.Recv()

		errorIsEOF = errors.Is(err, io.EOF)
		if errorIsEOF {
			continue
		}

		if err != nil {
			if err, ok := esdb.FromError(err); !ok {
				if err.Code() == esdb.ErrorCodeResourceNotFound && version == 1 {
					return []eventhorizon.Event{}, nil
				}
			}

			return nil, err
		}

		event, err := e.createEvent(esdbEvent)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, nil
}

func (e EventStore) createEvent(esdbEvent *esdb.ResolvedEvent) (eventhorizon.Event, error) {
	eventType := eventhorizon.EventType(esdbEvent.Event.EventType)
	eventData, err := eventhorizon.CreateEventData(eventType)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(esdbEvent.Event.Data, eventData); err != nil {
		return nil, err
	}

	return eventhorizon.NewEvent(
		eventType,
		eventData,
		esdbEvent.Event.CreatedDate,
	), nil
}
