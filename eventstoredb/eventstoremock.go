package eventstoredb

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
)

var _ eventhorizon.EventStore = &EventStoreMock{}

type EventStoreMock struct {
	SaveFn     func(ctx context.Context, events []eventhorizon.Event, originalVersion int) error
	LoadFn     func(ctx context.Context, uuid uuid.UUID) ([]eventhorizon.Event, error)
	LoadFromFn func(ctx context.Context, id uuid.UUID, version int) ([]eventhorizon.Event, error)
	CloseFn    func() error
}

func (mock *EventStoreMock) Save(ctx context.Context, events []eventhorizon.Event, originalVersion int) error {
	if mock != nil && mock.SaveFn != nil {
		return mock.SaveFn(ctx, events, originalVersion)
	}

	return nil
}

func (mock *EventStoreMock) Load(ctx context.Context, uuid uuid.UUID) ([]eventhorizon.Event, error) {
	if mock != nil && mock.LoadFn != nil {
		return mock.LoadFn(ctx, uuid)
	}

	return nil, nil
}

func (mock *EventStoreMock) LoadFrom(ctx context.Context, id uuid.UUID, version int) ([]eventhorizon.Event, error) {
	if mock != nil && mock.LoadFromFn != nil {
		return mock.LoadFromFn(ctx, id, version)
	}

	return nil, nil
}

func (mock *EventStoreMock) Close() error {
	if mock != nil && mock.CloseFn != nil {
		return mock.CloseFn()
	}

	return nil
}
