package movementtypeprojection

import (
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/eventhandler"
	"walletaccountant/eventstoredb"
	"walletaccountant/movementtype"
	"walletaccountant/subscription"
)

func ProjectionSubscribeEventStream(
	client eventstoredb.EventStorerer,
	projections *eventhandler.ProjectionRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	return subscription.SubscribeEventStreamForProjection(
		movementtype.AggregateType,
		eventhorizon.EventHandlerType(movementtype.AggregateType),
		client,
		projections,
		logger,
		lifecycle,
	)
}
