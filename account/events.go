package account

import (
	"github.com/looplab/eventhorizon"
	"time"
	"walletaccountant/definitions"
)

// Static type check that interface is implemented
var _ definitions.EventDataRegisters = &EventRegister{}

const (
	NewAccountRegistered = eventhorizon.EventType("new_account_registered")
	NextMonthStarted     = eventhorizon.EventType("next_month_started")
)

type EventRegister struct {
}

func NewEventRegister() *EventRegister {
	return &EventRegister{}
}

func (eventList *EventRegister) Registers() []definitions.EventDataRegister {
	return []definitions.EventDataRegister{
		{
			EventType: NewAccountRegistered,
			EventData: func() eventhorizon.EventData { return &NewAccountRegisteredData{} },
		},
		{
			EventType: NextMonthStarted,
			EventData: func() eventhorizon.EventData { return &NextMonthStartedData{} },
		},
	}
}

type NewAccountRegisteredData struct {
	AccountID           *Id        `json:"account_id"`
	BankName            string     `json:"bank_name"`
	Name                string     `json:"name"`
	AccountType         Type       `json:"type"`
	StartingBalance     float64    `json:"starting_balance"`
	StartingBalanceDate time.Time  `json:"starting_balance_date"`
	Currency            Currency   `json:"currency"`
	Notes               string     `json:"notes"`
	ActiveMonth         time.Month `json:"active_month"`
	ActiveYear          uint       `json:"active_year"`
}

type NextMonthStartedData struct {
	NextMonth time.Month `json:"next_month"`
	NextYear  uint       `json:"next_year"`
}
