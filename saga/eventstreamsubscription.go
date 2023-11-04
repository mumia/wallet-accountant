package saga

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/eventhandler"
	"walletaccountant/eventstoredb"
	"walletaccountant/subscription"
)

func AccountRegisterSagaSubscribeEventStream(
	client eventstoredb.EventStorerer,
	sagas *eventhandler.SagaRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	return subscription.SubscribeEventStreamForSaga(
		account.AggregateType,
		subscription.HandlerTypeForSaga(AccountRegisterSagaType.String()),
		client,
		sagas,
		logger,
		lifecycle,
	)
}

func AccountMonthEndedSagaSubscribeEventStream(
	client eventstoredb.EventStorerer,
	sagas *eventhandler.SagaRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	return subscription.SubscribeEventStreamForSaga(
		accountmonth.AggregateType,
		subscription.HandlerTypeForSaga(AccountMonthEndedSagaType.String()),
		client,
		sagas,
		logger,
		lifecycle,
	)
}
