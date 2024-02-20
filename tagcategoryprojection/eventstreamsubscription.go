package tagcategoryprojection

import (
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/eventhandler"
	"walletaccountant/eventstoredb"
	"walletaccountant/subscription"
	"walletaccountant/tagcategory"
)

func ProjectionSubscribeEventStream(
	client eventstoredb.EventStorerer,
	projections *eventhandler.ProjectionRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	return subscription.SubscribeEventStreamForProjection(
		tagcategory.AggregateType,
		eventhorizon.EventHandlerType(tagcategory.AggregateType),
		client,
		projections,
		logger,
		lifecycle,
	)
}
