package account

import (
	"context"
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
)

var _ eventhorizon.EventHandler = &Projection{}

type Projection struct {
	repository ReadModeler
}

func NewProjection(repository ReadModeler) (*Projection, error) {
	return &Projection{repository: repository}, nil
}

func (projection Projection) HandlerType() eventhorizon.EventHandlerType {
	return eventhorizon.EventHandlerType(AggregateType.String())
}

func (projection Projection) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	switch event.EventType() {
	case NewAccountRegistered:
		return projection.handleNewAccountRegistered(ctx, event)

	case NextMonthStarted:
		return projection.handleNextMonthStarted(ctx, event)
	}

	return nil
}

func (projection Projection) handleNewAccountRegistered(ctx context.Context, event eventhorizon.Event) error {
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

func (projection Projection) handleNextMonthStarted(ctx context.Context, event eventhorizon.Event) error {
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
