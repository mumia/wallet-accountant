package accountmonth

import (
	"context"
	"fmt"
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
)

var _ eventhorizon.EventHandler = &Projection{}
var _ ReadModelProjection = &Projection{}

type ReadModelProjection interface {
	eventhorizon.EventHandler
}

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
	case NewAccountMovementRegistered:
		return projection.handleNewAccountMovementRegistered(ctx, event)

	case MonthStarted:
		return projection.handleMonthStarted(ctx, event)

	case MonthEnded:
		return projection.handleMonthEnded(ctx, event)
	}

	return nil
}

func (projection Projection) handleNewAccountMovementRegistered(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*NewAccountMovementRegisteredData)
	if !ok {
		return definitions.EventDataTypeError(NewAccountMovementRegistered, event.EventType())
	}

	accountMonth, err := projection.repository.GetByAccountMonthId(ctx, eventData.AccountMonthId)
	if err != nil {
		return err
	}

	if accountMonth == nil {
		return fmt.Errorf("account month does not exist. AccountMonthId: %s", eventData.AccountMonthId)
	}

	return projection.repository.RegisterAccountMovement(
		ctx,
		accountMonth.AccountMonthId,
		eventData,
	)
}

func (projection Projection) handleMonthStarted(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*MonthStartedData)
	if !ok {
		return definitions.EventDataTypeError(MonthStarted, event.EventType())
	}

	return projection.repository.StartMonth(
		ctx,
		eventData.AccountMonthId,
		eventData.AccountId,
		eventData.StartBalance,
		eventData.Month,
		eventData.Year,
	)
}

func (projection Projection) handleMonthEnded(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*MonthEndedData)
	if !ok {
		return definitions.EventDataTypeError(MonthEnded, event.EventType())
	}

	return projection.repository.EndMonth(ctx, eventData.AccountMonthId)
}
