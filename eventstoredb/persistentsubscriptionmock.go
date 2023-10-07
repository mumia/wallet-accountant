package eventstoredb

import "github.com/EventStore/EventStore-Client-Go/v3/esdb"

var _ PersistentSubscriptioner = &PersistentSubscriptionMock{}

type PersistentSubscriptionMock struct {
	CloseFn func() error
	RecvFn  func() *esdb.PersistentSubscriptionEvent
	AckFn   func(messages ...*esdb.ResolvedEvent) error
	NackFn  func(
		reason string,
		action esdb.NackAction,
		messages ...*esdb.ResolvedEvent,
	) error
}

func (mock *PersistentSubscriptionMock) Close() error {
	if mock != nil && mock.CloseFn != nil {
		return mock.CloseFn()
	}

	return nil
}

func (mock *PersistentSubscriptionMock) Recv() *esdb.PersistentSubscriptionEvent {
	if mock != nil && mock.RecvFn != nil {
		return mock.RecvFn()
	}

	return nil
}

func (mock *PersistentSubscriptionMock) Ack(messages ...*esdb.ResolvedEvent) error {
	if mock != nil && mock.AckFn != nil {
		return mock.AckFn(messages...)
	}

	return nil
}

func (mock *PersistentSubscriptionMock) Nack(
	reason string,
	action esdb.NackAction,
	messages ...*esdb.ResolvedEvent,
) error {
	if mock != nil && mock.NackFn != nil {
		return mock.NackFn(reason, action, messages...)
	}

	return nil
}
