package tagcategory

import (
	"context"
	"errors"
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/clock"
	"walletaccountant/definitions"
)

var _ eventhorizon.Aggregate = &TagCategory{}

const AggregateType eventhorizon.AggregateType = "tagcategory"

type Id = uuid.UUID
type CategoryId = uuid.UUID

type TagCategory struct {
	*events.AggregateBase
	clock *clock.Clock

	name  string
	notes string
	tags  []*Tag
}

type Tag struct {
	tagId *Id
	name  string
	notes string
}

func (category *TagCategory) HandleCommand(ctx context.Context, command eventhorizon.Command) error {
	switch command.(type) {
	case *AddNewTagToNewCategory:
		if category.AggregateVersion() != 0 {
			return errors.New("tag category: is already registered")
		}
	default:
		if category.AggregateVersion() <= 0 {
			return errors.New("tag category: needs to be registered first")
		}
	}

	switch command := command.(type) {
	case *AddNewTagToNewCategory:
		category.AppendEvent(
			NewTagAddedToNewCategory,
			&NewTagAddedToNewCategoryData{
				TagCategoryId:    &command.TagCategoryId,
				TagCategoryName:  command.Name,
				TagCategoryNotes: command.Notes,
				TagId:            &command.Tag.TagId,
				TagName:          command.Tag.Name,
				TagNotes:         command.Tag.Notes,
			},
			category.clock.Now(),
		)

	case *AddNewTagToExistingCategory:
		category.AppendEvent(
			NewTagAddedToExistingCategory,
			&NewTagAddedToExistingCategoryData{
				TagId:         &command.TagId,
				TagCategoryId: &command.TagCategoryId,
				Name:          command.Name,
				Notes:         command.Notes,
			},
			category.clock.Now(),
		)

	default:
		return fmt.Errorf("no command matched. CommandType: %s", command.CommandType().String())
	}

	return nil
}

func (category *TagCategory) ApplyEvent(ctx context.Context, event eventhorizon.Event) error {
	switch event.EventType() {
	case NewTagAddedToNewCategory:
		eventData, ok := event.Data().(*NewTagAddedToNewCategoryData)
		if !ok {
			return definitions.EventDataTypeError(NewTagAddedToNewCategory, event.EventType())
		}

		category.name = eventData.TagCategoryName
		category.notes = eventData.TagCategoryNotes
		category.tags = []*Tag{
			{
				tagId: eventData.TagId,
				name:  eventData.TagName,
				notes: eventData.TagNotes,
			},
		}

	case NewTagAddedToExistingCategory:
		eventData, ok := event.Data().(*NewTagAddedToExistingCategoryData)
		if !ok {
			return definitions.EventDataTypeError(NewTagAddedToExistingCategory, event.EventType())
		}

		category.tags = append(
			category.tags,
			&Tag{
				tagId: eventData.TagId,
				name:  eventData.Name,
				notes: eventData.Notes,
			},
		)
	}

	return nil
}

func (category *TagCategory) CategoryId() *CategoryId {
	categoryId := CategoryId(category.EntityID())

	return &categoryId
}

func (category *TagCategory) Name() string {
	return category.name
}

func (category *TagCategory) Notes() string {
	return category.notes
}

func (category *TagCategory) Tags() []*Tag {
	return category.tags
}

func (tag *Tag) TagId() *Id {
	return tag.tagId
}

func (tag *Tag) Name() string {
	return tag.name
}

func (tag *Tag) Notes() string {
	return tag.notes
}
