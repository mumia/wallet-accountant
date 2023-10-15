package eventstoredb

import (
	"github.com/looplab/eventhorizon"
)

var _ EventStoreCreator = &EventStoreFactoryMock{}

type EventStoreFactoryMock struct {
	CreateEventStoreFn func(aggregateType eventhorizon.AggregateType, batchSize uint64) eventhorizon.EventStore
}

func (mock *EventStoreFactoryMock) CreateEventStore(
	aggregateType eventhorizon.AggregateType,
	batchSize uint64,
) eventhorizon.EventStore {
	if mock != nil && mock.CreateEventStoreFn != nil {
		return mock.CreateEventStoreFn(aggregateType, batchSize)
	}

	return nil
}
