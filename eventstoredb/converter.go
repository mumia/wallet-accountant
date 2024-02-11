package eventstoredb

import (
	"encoding/json"
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"strings"
)

func CreateEvent(esdbEvent *esdb.ResolvedEvent) (eventhorizon.Event, error) {
	streamSplitPosition := strings.Index(esdbEvent.Event.StreamID, "-")

	aggregateType := eventhorizon.AggregateType(esdbEvent.Event.StreamID[:streamSplitPosition])
	aggregateId := uuid.MustParse(esdbEvent.Event.StreamID[streamSplitPosition+1:])

	return CreateEventForAggregate(
		esdbEvent,
		aggregateType,
		aggregateId,
	)
}

func CreateEventForAggregate(
	esdbEvent *esdb.ResolvedEvent,
	aggregateType eventhorizon.AggregateType,
	aggregateId uuid.UUID,
) (eventhorizon.Event, error) {
	return createEvent(
		esdbEvent,
		eventhorizon.ForAggregate(aggregateType, aggregateId, int(esdbEvent.Event.EventNumber)+1),
	)
}

func createEvent(esdbEvent *esdb.ResolvedEvent, eventOptions ...eventhorizon.EventOption) (eventhorizon.Event, error) {
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
		eventOptions...,
	), nil
}
