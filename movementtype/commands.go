package movementtype

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/account"
	"walletaccountant/commands"
	"walletaccountant/eventstoredb"
	"walletaccountant/tagcategory"
)

var _ eventhorizon.Command = &RegisterNewMovementType{}

const (
	RegisterNewMovementTypeCommand = eventhorizon.CommandType("register_new_movement_type")
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
				Command:     &RegisterNewMovementType{},
				CommandType: RegisterNewMovementTypeCommand,
			},
		},
	)
}

type RegisterNewMovementType struct {
	MovementTypeId  Id                   `json:"movementTypeId"`
	Type            Type                 `json:"type"`
	AccountId       account.Id           `json:"accountId"`
	SourceAccountId *account.Id          `json:"sourceAccountId" eh:"optional"`
	Description     string               `json:"description"`
	Notes           *string              `json:"notes" eh:"optional"`
	TagIds          []*tagcategory.TagId `json:"tagIds"`
}

func (r RegisterNewMovementType) AggregateID() uuid.UUID {
	return uuid.UUID(r.MovementTypeId)
}

func (r RegisterNewMovementType) AggregateType() eventhorizon.AggregateType {
	return AggregateType
}

func (r RegisterNewMovementType) CommandType() eventhorizon.CommandType {
	return RegisterNewMovementTypeCommand
}
