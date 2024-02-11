package accountmonth

import (
	"github.com/looplab/eventhorizon"
	"time"
	"walletaccountant/account"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

var _ definitions.EventDataRegisters = &EventRegister{}

const (
	NewAccountMovementRegistered = eventhorizon.EventType("new_account_movement_registered")
	MonthStarted                 = eventhorizon.EventType("account_month_started")
	MonthEnded                   = eventhorizon.EventType("account_month_ended")
)

type EventRegister struct {
}

func NewEventRegister() *EventRegister {
	return &EventRegister{}
}

func (eventList *EventRegister) Registers() []definitions.EventDataRegister {
	return []definitions.EventDataRegister{
		{
			EventType: NewAccountMovementRegistered,
			EventData: func() eventhorizon.EventData { return &NewAccountMovementRegisteredData{} },
		},
		{
			EventType: MonthStarted,
			EventData: func() eventhorizon.EventData { return &MonthStartedData{} },
		},
		{
			EventType: MonthEnded,
			EventData: func() eventhorizon.EventData { return &MonthEndedData{} },
		},
	}
}

type NewAccountMovementRegisteredData struct {
	AccountMonthId    *Id                   `json:"account_month_id"`
	AccountMovementId *AccountMovementId    `json:"account_movement_id"`
	MovementTypeId    *movementtype.Id      `json:"movement_type_id"`
	Action            common.MovementAction `json:"action"`
	Amount            float32               `json:"amount"`
	Date              time.Time             `json:"date"`
	SourceAccountId   *account.Id           `json:"source_account_id"`
	Description       string                `json:"description"`
	Notes             *string               `json:"notes"`
	TagIds            []*tagcategory.TagId  `json:"tagIds"`
}

type MonthStartedData struct {
	AccountMonthId *Id         `json:"account_month_id"`
	AccountId      *account.Id `json:"account_id"`
	StartBalance   float32     `json:"start_balance"`
	Month          time.Month  `json:"month"`
	Year           uint        `json:"year"`
}

type MonthEndedData struct {
	AccountMonthId *Id         `json:"account_month_id"`
	AccountId      *account.Id `json:"account_id"`
	EndBalance     float32     `json:"end_balance"`
	Month          time.Month  `json:"month"`
	Year           uint        `json:"year"`
}
