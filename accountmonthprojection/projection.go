package accountmonthprojection

import (
	"context"
	"fmt"
	"github.com/looplab/eventhorizon"
	"walletaccountant/accountmonth"
	"walletaccountant/accountmonthreadmodel"
	"walletaccountant/definitions"
)

var _ eventhorizon.EventHandler = &Projection{}
var _ ReadModelProjection = &Projection{}

type ReadModelProjection interface {
	eventhorizon.EventHandler
}

type Projection struct {
	repository accountmonthreadmodel.ReadModeler
}

func NewProjection(repository accountmonthreadmodel.ReadModeler) (*Projection, error) {
	return &Projection{repository: repository}, nil
}

func (projection Projection) HandlerType() eventhorizon.EventHandlerType {
	return eventhorizon.EventHandlerType(accountmonth.AggregateType.String())
}

func (projection Projection) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	switch event.EventType() {
	case accountmonth.NewAccountMovementRegistered:
		return projection.handleNewAccountMovementRegistered(ctx, event)

	case accountmonth.MonthStarted:
		return projection.handleMonthStarted(ctx, event)

	case accountmonth.MonthEnded:
		return projection.handleMonthEnded(ctx, event)
	}

	return nil
}

func (projection Projection) handleNewAccountMovementRegistered(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*accountmonth.NewAccountMovementRegisteredData)
	if !ok {
		return definitions.EventDataTypeError(accountmonth.NewAccountMovementRegistered, event.EventType())
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
	eventData, ok := event.Data().(*accountmonth.MonthStartedData)
	if !ok {
		return definitions.EventDataTypeError(accountmonth.MonthStarted, event.EventType())
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
	eventData, ok := event.Data().(*accountmonth.MonthEndedData)
	if !ok {
		return definitions.EventDataTypeError(accountmonth.MonthEnded, event.EventType())
	}

	return projection.repository.EndMonth(ctx, eventData.AccountMonthId)
}
