package account

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/eventstoredb"
	"walletaccountant/projector"
	"walletaccountant/subscription"
)

func SubscribeEventStream(
	client eventstoredb.EventStorerer,
	eventMatcherHandlerRegistry *projector.EventMatcherHandlerRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	return subscription.SubscribeEventStream(AggregateType, client, eventMatcherHandlerRegistry, logger, lifecycle)
}
