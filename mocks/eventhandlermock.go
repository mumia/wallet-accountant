package mocks

import (
	"context"
	"github.com/looplab/eventhorizon"
)

var _ eventhorizon.EventHandler = &EventHandlerMock{}

type EventHandlerMock struct {
	HandlerTypeFn func() eventhorizon.EventHandlerType
	HandleEventFn func(ctx context.Context, event eventhorizon.Event) error
}

func (mock *EventHandlerMock) HandlerType() eventhorizon.EventHandlerType {
	if mock != nil && mock.HandlerTypeFn != nil {
		return mock.HandlerTypeFn()
	}

	return ""
}

func (mock *EventHandlerMock) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	if mock != nil && mock.HandleEventFn != nil {
		return mock.HandleEventFn(ctx, event)
	}

	return nil
}
