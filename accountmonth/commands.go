package accountmonth

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"time"
	"walletaccountant/account"
	"walletaccountant/commands"
	"walletaccountant/eventstoredb"
	"walletaccountant/movementtype"
)

var _ eventhorizon.Command = &RegisterNewAccountMovement{}
var _ eventhorizon.Command = &StartAccountMonth{}
var _ eventhorizon.Command = &EndAccountMonth{}

const (
	RegisterNewAccountMovementCommand = eventhorizon.CommandType("register_new_account_movement")
	StartAccountMonthCommand          = eventhorizon.CommandType("start_account_month")
	EndAccountMonthCommand            = eventhorizon.CommandType("end_account_month")
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
				Command:     &RegisterNewAccountMovement{},
				CommandType: RegisterNewAccountMovementCommand,
			},
			{
				Command:     &StartAccountMonth{},
				CommandType: StartAccountMonthCommand,
			},
			{
				Command:     &EndAccountMonth{},
				CommandType: EndAccountMonthCommand,
			},
		},
	)
}

type RegisterNewAccountMovement struct {
	AccountMonthId   Id                `json:"account_month"`
	MovementTypeId   movementtype.Id   `json:"movement_type_id"`
	MovementTypeType movementtype.Type `json:"movement_type"`
	Amount           float64           `json:"amount"`
	Date             time.Time         `json:"date"`
}

func (command RegisterNewAccountMovement) AggregateID() uuid.UUID {
	return uuid.UUID(command.AccountMonthId)
}

func (command RegisterNewAccountMovement) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (command RegisterNewAccountMovement) CommandType() eventhorizon.CommandType {
	return RegisterNewAccountMovementCommand
}

type StartAccountMonth struct {
	AccountMonthId Id         `json:"account_month"`
	AccountId      account.Id `json:"account_id"`
	StartBalance   float64    `json:"start_balance"`
	Month          time.Month `json:"month"`
	Year           uint       `json:"year"`
}

func (command StartAccountMonth) AggregateID() uuid.UUID {
	return uuid.UUID(command.AccountMonthId)
}

func (command StartAccountMonth) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (command StartAccountMonth) CommandType() eventhorizon.CommandType {
	return StartAccountMonthCommand
}

type EndAccountMonth struct {
	AccountMonthId Id         `json:"account_month"`
	AccountId      account.Id `json:"account_id"`
	EndBalance     float64    `json:"end_balance"`
	Month          time.Month `json:"month"`
	Year           uint       `json:"year"`
}

func (command EndAccountMonth) AggregateID() uuid.UUID {
	return uuid.UUID(command.AccountMonthId)
}

func (command EndAccountMonth) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (command EndAccountMonth) CommandType() eventhorizon.CommandType {
	return EndAccountMonthCommand
}
