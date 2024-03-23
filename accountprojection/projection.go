package accountprojection

import (
	"context"
	"github.com/looplab/eventhorizon"
	"go.uber.org/zap"
	"walletaccountant/account"
	"walletaccountant/accountreadmodel"
	"walletaccountant/definitions"
	"walletaccountant/websocket"
)

var _ eventhorizon.EventHandler = &Projection{}
var _ websocket.ModelUpdateNotifier = &Projection{}
var _ ReadModelProjection = &Projection{}

type ReadModelProjection interface {
	eventhorizon.EventHandler
}

type Projection struct {
	repository    accountreadmodel.ReadModeler
	updateChannel chan websocket.ModelUpdated
	log           *zap.Logger
}

func NewProjection(repository accountreadmodel.ReadModeler, log *zap.Logger) *Projection {
	return &Projection{
		repository:    repository,
		updateChannel: make(chan websocket.ModelUpdated),
		log:           log,
	}
}

func (projection *Projection) HandlerType() eventhorizon.EventHandlerType {
	return eventhorizon.EventHandlerType(account.AggregateType.String())
}

func (projection *Projection) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	var err error
	switch event.EventType() {
	case account.NewAccountRegistered:
		err = projection.handleNewAccountRegistered(ctx, event)

	case account.NextMonthStarted:
		err = projection.handleNextMonthStarted(ctx, event)
	}

	if err == nil {
		projection.updateChannel <- websocket.ModelUpdated{Event: event.EventType()}
	}

	return err
}

func (projection *Projection) UpdatedAggregate() eventhorizon.AggregateType {
	return account.AggregateType
}

func (projection *Projection) UpdateChannel() chan websocket.ModelUpdated {
	return projection.updateChannel
}

func (projection *Projection) handleNewAccountRegistered(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*account.NewAccountRegisteredData)
	if !ok {
		return definitions.EventDataTypeError(account.NewAccountRegistered, event.EventType())
	}

	account := accountreadmodel.Entity{
		AccountId:           eventData.AccountId,
		BankName:            eventData.BankName,
		Name:                eventData.Name,
		AccountType:         eventData.AccountType,
		StartingBalance:     eventData.StartingBalance,
		StartingBalanceDate: eventData.StartingBalanceDate,
		Currency:            eventData.Currency,
		Notes:               eventData.Notes,
		ActiveMonth: accountreadmodel.EntityActiveMonth{
			Month: eventData.ActiveMonth,
			Year:  eventData.ActiveYear,
		},
	}

	return projection.repository.Create(ctx, account)
}

func (projection *Projection) handleNextMonthStarted(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*account.NextMonthStartedData)
	if !ok {
		return definitions.EventDataTypeError(account.NextMonthStarted, event.EventType())
	}

	id := account.IdFromUUID(event.AggregateID())

	return projection.repository.UpdateActiveMonth(
		ctx,
		id,
		accountreadmodel.EntityActiveMonth{
			Month: eventData.NextMonth,
			Year:  eventData.NextYear,
		},
	)
}
