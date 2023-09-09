package account

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
)

type Mediator struct {
	commandHandler eventhorizon.CommandHandler
}

func NewMediator(commandHandler eventhorizon.CommandHandler) *Mediator {
	return &Mediator{commandHandler: commandHandler}
}

func (mediator Mediator) RegisterNewAccount(
	ctx *gin.Context,
	transferObject RegisterNewAccountTransferObject,
) (*uuid.UUID, error) {
	command, err := eventhorizon.CreateCommand(RegisterNewAccountCommand)
	if err != nil {
		return nil, err
	}

	registerNewAccountCommand, ok := command.(*RegisterNewAccount)
	if !ok {
		return nil, fmt.Errorf(
			"invalid command. Expected: %s Found: %s",
			RegisterNewAccountCommand,
			command.CommandType(),
		)
	}

	newAccountId := uuid.New()

	registerNewAccountCommand.AccountId = newAccountId
	registerNewAccountCommand.BankName = transferObject.BankName
	registerNewAccountCommand.Name = transferObject.Name
	registerNewAccountCommand.AccountType = Type(transferObject.AccountType)
	registerNewAccountCommand.StartingBalance = transferObject.StartingBalance
	registerNewAccountCommand.StartingBalanceDate = transferObject.StartingBalanceDate
	registerNewAccountCommand.Currency = Currency(transferObject.Currency)
	registerNewAccountCommand.Notes = transferObject.Notes

	err = mediator.commandHandler.HandleCommand(ctx, registerNewAccountCommand)
	if err != nil {
		return nil, err
	}

	return &newAccountId, nil
}
