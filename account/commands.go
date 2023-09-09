package account

import (
	"errors"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/commandhandler/bus"
	"github.com/looplab/eventhorizon/uuid"
	"time"
	"walletaccountant/definitions"
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
	eventStoreFactory eventstoredb.EventStoreFactory,
	commandHandler eventhorizon.CommandHandler,
) error {
	busCommandHandler, ok := commandHandler.(*bus.CommandHandler)
	if !ok {
		return errors.New("")
	}

	definitions.RegisterCommands(
		[]func() eventhorizon.Command{
			func() eventhorizon.Command { return &RegisterNewAccount{} },
			func() eventhorizon.Command { return &StartNextMonth{} },
		},
	)

	eventStore := eventStoreFactory(AggregateType)

	aggregateStore, err := events.NewAggregateStore(eventStore)
	if err != nil {
		return err
	}

	return definitions.RegisterCommandTypes(
		aggregateStore,
		busCommandHandler,
		AggregateType,
		[]eventhorizon.CommandType{
			RegisterNewAccountCommand,
			StartNextMonthCommand,
		},
	)
}

type RegisterNewAccount struct {
	AccountId           uuid.UUID `json:"account_id"`
	BankName            string    `json:"bank_name"`
	Name                string    `json:"name"`
	AccountType         Type      `json:"type"`
	StartingBalance     float64   `json:"starting_balance"`
	StartingBalanceDate time.Time `json:"starting_balance_date"`
	Currency            Currency  `json:"currency"`
	Notes               string    `json:"notes"`
}

func (r RegisterNewAccount) AggregateID() uuid.UUID {
	return r.AccountId
}

func (r RegisterNewAccount) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (r RegisterNewAccount) CommandType() eventhorizon.CommandType {
	return RegisterNewAccountCommand
}

type StartNextMonth struct {
	AccountId uuid.UUID `json:"account_id"`
}

func (s StartNextMonth) AggregateID() uuid.UUID {
	return s.AccountId
}

func (s StartNextMonth) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (s StartNextMonth) CommandType() eventhorizon.CommandType {
	return StartNextMonthCommand
}
