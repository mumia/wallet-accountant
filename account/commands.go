package account

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"time"
	"walletaccountant/commands"
	"walletaccountant/eventstoredb"
)

// Static type check that interface is implemented
var _ eventhorizon.Command = &RegisterNewAccount{}
var _ eventhorizon.Command = &StartNextMonth{}

const (
	RegisterNewAccountCommand = eventhorizon.CommandType("register_new_account")
	StartNextMonthCommand     = eventhorizon.CommandType("start_next_month")
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
				Command:     &RegisterNewAccount{},
				CommandType: RegisterNewAccountCommand,
			},
			{
				Command:     &StartNextMonth{},
				CommandType: StartNextMonthCommand,
			},
		},
	)
}

type RegisterNewAccount struct {
	AccountId           Id        `json:"account_id"`
	BankName            string    `json:"bank_name"`
	Name                string    `json:"name"`
	AccountType         Type      `json:"type"`
	StartingBalance     float64   `json:"starting_balance"`
	StartingBalanceDate time.Time `json:"starting_balance_date"`
	Currency            Currency  `json:"currency"`
	Notes               *string   `json:"notes" eh:"optional"`
}

func (r RegisterNewAccount) AggregateID() uuid.UUID {
	return uuid.UUID(r.AccountId)
}

func (r RegisterNewAccount) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (r RegisterNewAccount) CommandType() eventhorizon.CommandType {
	return RegisterNewAccountCommand
}

type StartNextMonth struct {
	AccountId Id      `json:"account_id"`
	Balance   float64 `json:"balance"`
}

func (s StartNextMonth) AggregateID() uuid.UUID {
	return uuid.UUID(s.AccountId)
}

func (s StartNextMonth) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (s StartNextMonth) CommandType() eventhorizon.CommandType {
	return StartNextMonthCommand
}
