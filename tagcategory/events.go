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
	TagCategoryId    *CategoryId `json:"tagCategoryId"`
	TagCategoryName  string      `json:"TagCategoryName"`
	TagCategoryNotes string      `json:"TagCategoryNotes"`
	TagId            *Id         `json:"tagId"`
	TagName          string      `json:"tagName"`
	TagNotes         string      `json:"tagNotes"`
}

type NewTagAddedToExistingCategoryData struct {
	TagCategoryId *CategoryId `json:"tagCategoryId"`
	TagId         *Id         `json:"tagId"`
	Name          string      `json:"name"`
	Notes         string      `json:"notes"`
}
