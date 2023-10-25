package movementtype

import (
	"context"
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
	case NewMovementTypeRegistered:
		return projection.handleNewMovementTypeRegistered(ctx, event)
	}

	return nil
}

func (projection Projection) handleNewMovementTypeRegistered(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*NewMovementTypeRegisteredData)
	if !ok {
		return definitions.EventDataTypeError(NewMovementTypeRegistered, event.EventType())
	}

	account := Entity{
		MovementTypeId:  eventData.MovementTypeId,
		Type:            eventData.Type,
		AccountId:       eventData.AccountId,
		SourceAccountId: eventData.SourceAccountId,
		Description:     eventData.Description,
		Notes:           eventData.Notes,
		Tags:            eventData.TagIds,
	}

	return projection.repository.Create(ctx, &account)
}
