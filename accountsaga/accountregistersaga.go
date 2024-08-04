package accountsaga

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/eventhandler/saga"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/ledger"
)

var _ saga.Saga = &AccountRegisterSaga{}
var _ definitions.SagaProvider = &AccountRegisterSaga{}

const AccountRegisterSagaType saga.Type = "AccountRegisterSaga"

type AccountRegisterSaga struct {
}

func NewAccountRegisterSaga() *AccountRegisterSaga {
	return &AccountRegisterSaga{}
}

func (saga *AccountRegisterSaga) Matcher() eventhorizon.MatchEvents {
	return eventhorizon.MatchEvents{
		account.NewAccountRegistered,
	}
}

func (saga *AccountRegisterSaga) SagaType() saga.Type {
	return AccountRegisterSagaType
}

func (saga *AccountRegisterSaga) RunSaga(
	ctx context.Context,
	event eventhorizon.Event,
	handler eventhorizon.CommandHandler,
) error {
	switch event.EventType() {
	case account.NewAccountRegistered:
		eventData, ok := event.Data().(*account.NewAccountRegisteredData)
		if !ok {
			return definitions.EventDataTypeError(account.NewAccountRegistered, event.EventType())
		}

		return saga.handleNewAccountRegistered(ctx, handler, eventData)
	}

	return nil
}

func (saga *AccountRegisterSaga) handleNewAccountRegistered(
	ctx context.Context,
	handler eventhorizon.CommandHandler,
	eventData *account.NewAccountRegisteredData,
) error {
	accountMonthId, err := ledger.IdGenerate(
		eventData.AccountId,
		eventData.ActiveMonth,
		eventData.ActiveYear,
	)
	if err != nil {
		return err
	}

	return handler.HandleCommand(
		ctx,
		&ledger.StartAccountMonth{
			AccountMonthId: *accountMonthId,
			AccountId:      *eventData.AccountId,
			StartBalance:   eventData.StartingBalance,
			Month:          eventData.ActiveMonth,
			Year:           eventData.ActiveYear,
		},
	)
}
