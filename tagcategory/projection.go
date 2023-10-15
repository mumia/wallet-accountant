package tagcategory

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
	case NewTagAddedToNewCategory:
		return projection.handleNewTagAddedToNewCategory(ctx, event)

	case NewTagAddedToExistingCategory:
		return projection.handleNewTagAddedToExistingCategory(ctx, event)
	}

	return nil
}

func (projection Projection) handleNewTagAddedToNewCategory(ctx context.Context, event eventhorizon.Event) error {
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

func (projection Projection) handleNewTagAddedToExistingCategory(ctx context.Context, event eventhorizon.Event) error {
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
