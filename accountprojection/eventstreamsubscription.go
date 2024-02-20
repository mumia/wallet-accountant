package accountprojection

import (
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/account"
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
		account.AggregateType,
		eventhorizon.EventHandlerType(account.AggregateType),
		client,
		projections,
		logger,
		lifecycle,
	)
}
