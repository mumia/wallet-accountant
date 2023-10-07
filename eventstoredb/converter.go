package eventstoredb

import (
	"encoding/json"
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/looplab/eventhorizon"
)

func CreateEvent(esdbEvent *esdb.ResolvedEvent) (eventhorizon.Event, error) {
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
