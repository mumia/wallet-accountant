package eventstoredb

import (
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/looplab/eventhorizon"
)

var _ EventStoreCreator = &EventStoreFactory{}

type EventStoreCreator interface {
	CreateEventStore(aggregateType eventhorizon.AggregateType, batchSize uint64) eventhorizon.EventStore
}

type EventStoreFactory struct {
	client EventStorerer
}

func NewEventStoreFactory(client EventStorerer) (*EventStoreFactory, error) {
	return &EventStoreFactory{client: client}, nil
}

func (factory *EventStoreFactory) CreateEventStore(
	aggregateType eventhorizon.AggregateType,
	batchSize uint64,
) eventhorizon.EventStore {
	return &EventStore{
		client:        factory.client,
		contentType:   esdb.ContentTypeJson,
		aggregateType: aggregateType,
		batchSize:     batchSize,
	}
}
