package tagcategory

import (
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
)

var _ definitions.EventDataRegisters = &EventRegister{}

const (
	NewTagAddedToNewCategory      = eventhorizon.EventType("new_tag_added_to_new_category")
	NewTagAddedToExistingCategory = eventhorizon.EventType("new_tag_added_to_existing_category")
)

type EventRegister struct {
}

func NewEventRegister() *EventRegister {
	return &EventRegister{}
}

func (eventList *EventRegister) Registers() []definitions.EventDataRegister {
	return []definitions.EventDataRegister{
		{
			EventType: NewTagAddedToNewCategory,
			EventData: func() eventhorizon.EventData { return &NewTagAddedToNewCategoryData{} },
		},
		{
			EventType: NewTagAddedToExistingCategory,
			EventData: func() eventhorizon.EventData { return &NewTagAddedToExistingCategoryData{} },
		},
	}
}

type NewTagAddedToNewCategoryData struct {
	TagCategoryId    *Id    `json:"tag_category_id"`
	TagCategoryName  string `json:"tag_category_name"`
	TagCategoryNotes string `json:"tag_category_notes"`
	TagId            *TagId `json:"tag_id"`
	TagName          string `json:"tag_name"`
	TagNotes         string `json:"tag_notes"`
}

type NewTagAddedToExistingCategoryData struct {
	TagCategoryId *Id    `json:"tag_category_id"`
	TagId         *TagId `json:"tag_id"`
	Name          string `json:"name"`
	Notes         string `json:"notes"`
}
