package tagcategoryprojection

import (
	"context"
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategoryreadmodel"
	"walletaccountant/websocket"
)

var _ eventhorizon.EventHandler = &Projection{}
var _ websocket.ModelUpdateNotifier = &Projection{}
var _ ReadModelProjection = &Projection{}

type ReadModelProjection interface {
	eventhorizon.EventHandler
}

type Projection struct {
	repository    tagcategoryreadmodel.ReadModeler
	updateChannel chan websocket.ModelUpdated
}

func NewProjection(repository tagcategoryreadmodel.ReadModeler) (*Projection, error) {
	return &Projection{
		repository:    repository,
		updateChannel: make(chan websocket.ModelUpdated),
	}, nil
}

func (projection *Projection) HandlerType() eventhorizon.EventHandlerType {
	return eventhorizon.EventHandlerType(tagcategory.AggregateType.String())
}

func (projection *Projection) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	var err error
	switch event.EventType() {
	case tagcategory.NewTagAddedToNewCategory:
		err = projection.handleNewTagAddedToNewCategory(ctx, event)

	case tagcategory.NewTagAddedToExistingCategory:
		err = projection.handleNewTagAddedToExistingCategory(ctx, event)
	}

	if err == nil {
		projection.updateChannel <- websocket.ModelUpdated{Event: event.EventType()}
	}

	return nil
}

func (projection *Projection) UpdatedAggregate() eventhorizon.AggregateType {
	return tagcategory.AggregateType
}

func (projection *Projection) UpdateChannel() chan websocket.ModelUpdated {
	return projection.updateChannel
}

func (projection *Projection) handleNewTagAddedToNewCategory(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*tagcategory.NewTagAddedToNewCategoryData)
	if !ok {
		return definitions.EventDataTypeError(tagcategory.NewTagAddedToNewCategory, event.EventType())
	}

	newTagAndCategory := &tagcategoryreadmodel.CategoryEntity{
		TagCategoryId: eventData.TagCategoryId,
		Name:          eventData.TagCategoryName,
		Notes:         eventData.TagCategoryNotes,
		Tags: []*tagcategoryreadmodel.Entity{
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
	eventData, ok := event.Data().(*tagcategory.NewTagAddedToExistingCategoryData)
	if !ok {
		return definitions.EventDataTypeError(tagcategory.NewTagAddedToExistingCategory, event.EventType())
	}

	return projection.repository.AddNewTagToCategory(
		ctx,
		eventData.TagCategoryId,
		&tagcategoryreadmodel.Entity{
			TagId: eventData.TagId,
			Name:  eventData.Name,
			Notes: eventData.Notes,
		},
	)
}
