package ledgersaga

import (
	"context"
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/eventhandler/saga"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/ledger"
)

var _ saga.Saga = &AccountMonthEndedSaga{}
var _ definitions.SagaProvider = &AccountMonthEndedSaga{}

const AccountMonthEndedSagaType saga.Type = "AccountMonthEndedSaga"

type AccountMonthEndedSaga struct {
	accountAggregateStore eventhorizon.AggregateStore
}

func NewAccountMonthEndedSaga(eventStoreFactory eventstoredb.EventStoreCreator) (*AccountMonthEndedSaga, error) {
	eventStore := eventStoreFactory.CreateEventStore(account.AggregateType, 100)
	aggregateStore, err := events.NewAggregateStore(eventStore)
	if err != nil {
		return nil, err
	}

	return &AccountMonthEndedSaga{
		accountAggregateStore: aggregateStore,
	}, nil
}

func (saga *AccountMonthEndedSaga) Matcher() eventhorizon.MatchEvents {
	return eventhorizon.MatchEvents{
		ledger.MonthEnded,
	}
}

func (saga *AccountMonthEndedSaga) SagaType() saga.Type {
	return AccountMonthEndedSagaType
}

func (saga *AccountMonthEndedSaga) RunSaga(
	ctx context.Context,
	event eventhorizon.Event,
	handler eventhorizon.CommandHandler,
) error {
	switch event.EventType() {
	case ledger.MonthEnded:
		eventData, ok := event.Data().(*ledger.MonthEndedData)
		if !ok {
			return definitions.EventDataTypeError(ledger.MonthEnded, event.EventType())
		}

		return saga.handleNewAccountMonthEnded(ctx, handler, eventData)
	}

	return nil
}

func (saga *AccountMonthEndedSaga) handleNewAccountMonthEnded(
	ctx context.Context,
	handler eventhorizon.CommandHandler,
	eventData *ledger.MonthEndedData,
) error {
	err := handler.HandleCommand(
		ctx,
		&account.StartNextMonth{
			AccountId: *eventData.AccountId,
			Balance:   eventData.EndBalance,
		},
	)
	if err != nil {
		return err
	}

	aggregate, err := saga.accountAggregateStore.Load(ctx, account.AggregateType, uuid.UUID(*eventData.AccountId))
	if err != nil {
		return err
	}

	accountAggregate, ok := aggregate.(*account.Account)
	if !ok {
		return fmt.Errorf("account aggregate is of wrong type. AccountId: %s", eventData.AccountId.String())
	}

	activeMonth := accountAggregate.ActiveMonth()

	accountMonthId, err := ledger.IdGenerate(
		eventData.AccountId,
		activeMonth.Month(),
		activeMonth.Year(),
	)

	return handler.HandleCommand(
		ctx,
		&ledger.StartAccountMonth{
			AccountMonthId: *accountMonthId,
			AccountId:      *eventData.AccountId,
			StartBalance:   eventData.EndBalance,
			Month:          activeMonth.Month(),
			Year:           activeMonth.Year(),
		},
	)
}
