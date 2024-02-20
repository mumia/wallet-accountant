package accountmonthprojection

import (
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/accountmonth"
	"walletaccountant/eventhandler"
	"walletaccountant/eventstoredb"
	"walletaccountant/subscription"
)

func ProjectionSubscribeEventStream(
	client eventstoredb.EventStorerer,
	projections *eventhandler.ProjectionRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	return subscription.SubscribeEventStreamForProjection(
		accountmonth.AggregateType,
		eventhorizon.EventHandlerType(accountmonth.AggregateType),
		client,
		projections,
		logger,
		lifecycle,
	)
}
