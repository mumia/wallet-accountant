package eventstoredb

import "github.com/EventStore/EventStore-Client-Go/v3/esdb"

type PersistentSubscriptioner interface {
	Close() error
	Recv() *esdb.PersistentSubscriptionEvent
	Ack(messages ...*esdb.ResolvedEvent) error
	Nack(
		reason string,
		action esdb.NackAction,
		messages ...*esdb.ResolvedEvent,
	) error
}
