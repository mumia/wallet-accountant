package ledgerprojection

import (
	"context"
	"fmt"
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
	"walletaccountant/ledger"
	"walletaccountant/ledgerreadmodel"
)

var _ eventhorizon.EventHandler = &Projection{}
var _ ReadModelProjection = &Projection{}

type ReadModelProjection interface {
	eventhorizon.EventHandler
}

type Projection struct {
	repository ledgerreadmodel.ReadModeler
}

func NewProjection(repository ledgerreadmodel.ReadModeler) (*Projection, error) {
	return &Projection{repository: repository}, nil
}

func (projection Projection) HandlerType() eventhorizon.EventHandlerType {
	return eventhorizon.EventHandlerType(ledger.AggregateType.String())
}

func (projection Projection) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	switch event.EventType() {
	case ledger.NewAccountMovementRegistered:
		return projection.handleNewAccountMovementRegistered(ctx, event)

	case ledger.MonthStarted:
		return projection.handleMonthStarted(ctx, event)

	case ledger.MonthEnded:
		return projection.handleMonthEnded(ctx, event)
	}

	return nil
}

func (projection Projection) handleNewAccountMovementRegistered(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*ledger.NewAccountMovementRegisteredData)
	if !ok {
		return definitions.EventDataTypeError(ledger.NewAccountMovementRegistered, event.EventType())
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
	eventData, ok := event.Data().(*ledger.MonthStartedData)
	if !ok {
		return definitions.EventDataTypeError(ledger.MonthStarted, event.EventType())
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
	eventData, ok := event.Data().(*ledger.MonthEndedData)
	if !ok {
		return definitions.EventDataTypeError(ledger.MonthEnded, event.EventType())
	}

	return projection.repository.EndMonth(ctx, eventData.AccountMonthId)
}
