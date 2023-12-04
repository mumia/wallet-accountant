package tagcategory

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/commands"
	"walletaccountant/eventstoredb"
)

var _ eventhorizon.Command = &AddNewTagToNewCategory{}

const (
	AddNewTagToNewCategoryCommand      = eventhorizon.CommandType("add_new_tag_to_new_category")
	AddNewTagToExistingCategoryCommand = eventhorizon.CommandType("add_new_tag_to_existing_category")
)

func RegisterCommandHandler(
	eventStoreFactory eventstoredb.EventStoreCreator,
	commandHandler eventhorizon.CommandHandler,
) error {
	return commands.RegisterCommandTypes(
		eventStoreFactory,
		commandHandler,
		AggregateType,
		[]commands.CommandAndType{
			{
				Command:     &AddNewTagToNewCategory{},
				CommandType: AddNewTagToNewCategoryCommand,
			},
			{
				Command:     &AddNewTagToExistingCategory{},
				CommandType: AddNewTagToExistingCategoryCommand,
			},
		},
	)
}

type AddNewTagToNewCategory struct {
	TagCategoryId Id      `json:"tag_category_id"`
	Name          string  `json:"name"`
	Notes         *string `json:"notes" eh:"optional"`
	Tag           NewTag  `json:"tag"`
}

type NewTag struct {
	TagId TagId   `json:"tag_id"`
	Name  string  `json:"name"`
	Notes *string `json:"notes" eh:"optional"`
}

func (r AddNewTagToNewCategory) AggregateID() uuid.UUID {
	return uuid.UUID(r.TagCategoryId)
}

func (r AddNewTagToNewCategory) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (r AddNewTagToNewCategory) CommandType() eventhorizon.CommandType {
	return AddNewTagToNewCategoryCommand
}

type AddNewTagToExistingCategory struct {
	TagId         TagId   `json:"tag_id"`
	TagCategoryId Id      `json:"tag_category_id"`
	Name          string  `json:"name"`
	Notes         *string `json:"notes" eh:"optional"`
}

func (r AddNewTagToExistingCategory) AggregateID() uuid.UUID {
	return uuid.UUID(r.TagCategoryId)
}

func (r AddNewTagToExistingCategory) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (r AddNewTagToExistingCategory) CommandType() eventhorizon.CommandType {
	return AddNewTagToExistingCategoryCommand
}
