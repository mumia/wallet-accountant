package movementtype

import (
	"github.com/looplab/eventhorizon"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var _ definitions.EventDataRegisters = &EventRegister{}

const (
	NewMovementTypeRegistered = eventhorizon.EventType("new_movement_type_registered")
)

type EventRegister struct {
}

func NewEventRegister() *EventRegister {
	return &EventRegister{}
}

func (eventList *EventRegister) Registers() []definitions.EventDataRegister {
	return []definitions.EventDataRegister{
		{
			EventType: NewMovementTypeRegistered,
			EventData: func() eventhorizon.EventData { return &NewMovementTypeRegisteredData{} },
		},
	}
}

type NewMovementTypeRegisteredData struct {
	MovementTypeId  *Id                  `json:"movement_type_id"`
	Type            Type                 `json:"type"`
	AccountId       *account.Id          `json:"account_id"`
	SourceAccountId *account.Id          `json:"source_account_id"`
	Description     string               `json:"description"`
	Notes           *string              `json:"notes"`
	TagIds          []*tagcategory.TagId `json:"tag_ids"`
}
