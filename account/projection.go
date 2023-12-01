package account

import (
	"context"
	"github.com/looplab/eventhorizon"
	"go.uber.org/zap"
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
	repository    ReadModeler
	updateChannel chan websocket.ModelUpdated
	log           *zap.Logger
}

func NewProjection(repository ReadModeler, log *zap.Logger) *Projection {
	return &Projection{
		repository:    repository,
		updateChannel: make(chan websocket.ModelUpdated),
		log:           log,
	}
}

func (projection *Projection) HandlerType() eventhorizon.EventHandlerType {
	return eventhorizon.EventHandlerType(AggregateType.String())
}

func (projection *Projection) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	var err error
	switch event.EventType() {
	case NewAccountRegistered:
		err = projection.handleNewAccountRegistered(ctx, event)

	case NextMonthStarted:
		err = projection.handleNextMonthStarted(ctx, event)
	}

	if err == nil {
		projection.updateChannel <- websocket.ModelUpdated{Event: event.EventType()}
	}

	return err
}

func (projection *Projection) UpdatedAggregate() eventhorizon.AggregateType {
	return AggregateType
}

func (projection *Projection) UpdateChannel() chan websocket.ModelUpdated {
	return projection.updateChannel
}

func (projection *Projection) handleNewAccountRegistered(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*NewAccountRegisteredData)
	if !ok {
		return definitions.EventDataTypeError(NewAccountRegistered, event.EventType())
	}

	account := Entity{
		AccountId:           eventData.AccountID,
		BankName:            eventData.BankName,
		Name:                eventData.Name,
		AccountType:         eventData.AccountType,
		StartingBalance:     eventData.StartingBalance,
		StartingBalanceDate: eventData.StartingBalanceDate,
		Currency:            eventData.Currency,
		Notes:               eventData.Notes,
		ActiveMonth: EntityActiveMonth{
			Month: eventData.ActiveMonth,
			Year:  eventData.ActiveYear,
		},
	}

	return projection.repository.Create(ctx, account)
}

func (projection *Projection) handleNextMonthStarted(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*NextMonthStartedData)
	if !ok {
		return definitions.EventDataTypeError(NextMonthStarted, event.EventType())
	}

	id := Id(event.AggregateID())

	return projection.repository.UpdateActiveMonth(
		ctx,
		&id,
		EntityActiveMonth{
			Month: eventData.NextMonth,
			Year:  eventData.NextYear,
		},
	)
}
