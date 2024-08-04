package saga

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/account"
	"walletaccountant/accountsaga"
	"walletaccountant/eventhandler"
	"walletaccountant/eventstoredb"
	"walletaccountant/importfile"
	"walletaccountant/importfilesaga"
	"walletaccountant/ledger"
	"walletaccountant/ledgersaga"
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
		subscription.HandlerTypeForSaga(accountsaga.AccountRegisterSagaType.String()),
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
		ledger.AggregateType,
		subscription.HandlerTypeForSaga(ledgersaga.AccountMonthEndedSagaType.String()),
		client,
		sagas,
		logger,
		lifecycle,
	)
}

func ImportFileDataRowVerifiedSagaSubscribeEventStream(
	client eventstoredb.EventStorerer,
	sagas *eventhandler.SagaRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	return subscription.SubscribeEventStreamForSaga(
		importfile.AggregateType,
		subscription.HandlerTypeForSaga(importfilesaga.ImportFileDataRowVerifiedSagaType.String()),
		client,
		sagas,
		logger,
		lifecycle,
	)
}
