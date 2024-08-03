package importfileprojection

import (
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/eventhandler"
	"walletaccountant/eventstoredb"
	"walletaccountant/importfile"
	"walletaccountant/subscription"
)

func ProjectionSubscribeEventStream(
	client eventstoredb.EventStorerer,
	projections *eventhandler.ProjectionRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	return subscription.SubscribeEventStreamForProjection(
		importfile.AggregateType,
		eventhorizon.EventHandlerType(importfile.AggregateType),
		client,
		projections,
		logger,
		lifecycle,
	)
}
