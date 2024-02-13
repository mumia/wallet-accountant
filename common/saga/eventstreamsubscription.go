package saga

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/account"
	"walletaccountant/account/saga"
	"walletaccountant/accountmonth"
	saga2 "walletaccountant/accountmonth/saga"
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
		subscription.HandlerTypeForSaga(saga.AccountRegisterSagaType.String()),
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
		subscription.HandlerTypeForSaga(saga2.AccountMonthEndedSagaType.String()),
		client,
		sagas,
		logger,
		lifecycle,
	)
}
