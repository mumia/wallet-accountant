package tagcategory

import (
	"context"
	"github.com/looplab/eventhorizon"
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
}

func NewProjection(repository ReadModeler) (*Projection, error) {
	return &Projection{
		repository:    repository,
		updateChannel: make(chan websocket.ModelUpdated),
	}, nil
}

func (projection *Projection) HandlerType() eventhorizon.EventHandlerType {
	return eventhorizon.EventHandlerType(AggregateType.String())
}

func (projection *Projection) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	var err error
	switch event.EventType() {
	case NewTagAddedToNewCategory:
		err = projection.handleNewTagAddedToNewCategory(ctx, event)

	case NewTagAddedToExistingCategory:
		err = projection.handleNewTagAddedToExistingCategory(ctx, event)
	}

	if err == nil {
		projection.updateChannel <- websocket.ModelUpdated{Event: event.EventType()}
	}

	return nil
}

func (projection *Projection) UpdatedAggregate() eventhorizon.AggregateType {
	return AggregateType
}

func (projection *Projection) UpdateChannel() chan websocket.ModelUpdated {
	return projection.updateChannel
}

func (projection *Projection) handleNewTagAddedToNewCategory(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*NewTagAddedToNewCategoryData)
	if !ok {
		return definitions.EventDataTypeError(NewTagAddedToNewCategory, event.EventType())
	}

	newTagAndCategory := &CategoryEntity{
		TagCategoryId: eventData.TagCategoryId,
		Name:          eventData.TagCategoryName,
		Notes:         eventData.TagCategoryNotes,
		Tags: []*Entity{
			{
				TagId: eventData.TagId,
				Name:  eventData.TagName,
				Notes: eventData.TagNotes,
			},
		},
	}

	return projection.repository.AddNewTagAndCategory(ctx, newTagAndCategory)
}

func (projection *Projection) handleNewTagAddedToExistingCategory(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*NewTagAddedToExistingCategoryData)
	if !ok {
		return definitions.EventDataTypeError(NewTagAddedToExistingCategory, event.EventType())
	}

	return projection.repository.AddNewTagToCategory(
		ctx,
		eventData.TagCategoryId,
		&Entity{
			TagId: eventData.TagId,
			Name:  eventData.Name,
			Notes: eventData.Notes,
		},
	)
}
