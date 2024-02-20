package movementtypeprojection

import (
	"context"
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypereadmodel"
	"walletaccountant/websocket"
)

var _ eventhorizon.EventHandler = &Projection{}
var _ websocket.ModelUpdateNotifier = &Projection{}
var _ ReadModelProjection = &Projection{}

type ReadModelProjection interface {
	eventhorizon.EventHandler
}

type Projection struct {
	repository    movementtypereadmodel.ReadModeler
	updateChannel chan websocket.ModelUpdated
}

func NewProjection(repository movementtypereadmodel.ReadModeler) (*Projection, error) {
	return &Projection{
		repository:    repository,
		updateChannel: make(chan websocket.ModelUpdated),
	}, nil
}

func (projection *Projection) HandlerType() eventhorizon.EventHandlerType {
	return eventhorizon.EventHandlerType(movementtype.AggregateType.String())
}

func (projection *Projection) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	var err error
	switch event.EventType() {
	case movementtype.NewMovementTypeRegistered:
		err = projection.handleNewMovementTypeRegistered(ctx, event)
	}

	if err == nil {
		projection.updateChannel <- websocket.ModelUpdated{Event: event.EventType()}
	}

	return nil
}

func (projection *Projection) UpdatedAggregate() eventhorizon.AggregateType {
	return movementtype.AggregateType
}

func (projection *Projection) UpdateChannel() chan websocket.ModelUpdated {
	return projection.updateChannel
}

func (projection *Projection) handleNewMovementTypeRegistered(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*movementtype.NewMovementTypeRegisteredData)
	if !ok {
		return definitions.EventDataTypeError(movementtype.NewMovementTypeRegistered, event.EventType())
	}

	account := movementtypereadmodel.Entity{
		MovementTypeId:  eventData.MovementTypeId,
		Action:          eventData.Action,
		AccountId:       eventData.AccountId,
		SourceAccountId: eventData.SourceAccountId,
		Description:     eventData.Description,
		Notes:           eventData.Notes,
		Tags:            eventData.TagIds,
	}

	return projection.repository.Create(ctx, &account)
}
