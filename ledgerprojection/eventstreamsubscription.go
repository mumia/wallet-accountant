package ledgerprojection

import (
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/eventhandler"
	"walletaccountant/eventstoredb"
	"walletaccountant/ledger"
	"walletaccountant/subscription"
)

func ProjectionSubscribeEventStream(
	client eventstoredb.EventStorerer,
	projections *eventhandler.ProjectionRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	return subscription.SubscribeEventStreamForProjection(
		ledger.AggregateType,
		eventhorizon.EventHandlerType(ledger.AggregateType),
		client,
		projections,
		logger,
		lifecycle,
	)
}
