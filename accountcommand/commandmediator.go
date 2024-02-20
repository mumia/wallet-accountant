package accountcommand

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"go.mongodb.org/mongo-driver/mongo"
	"walletaccountant/account"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
)

var _ CommandMediatorer = &CommandMediator{}

type CommandMediatorer interface {
	RegisterNewAccount(
		ctx *gin.Context,
		transferObject RegisterNewAccountTransferObject,
	) (*account.Id, *definitions.WalletAccountantError)
	StartNextMonth(ctx *gin.Context, accountId *account.Id) *definitions.WalletAccountantError
}

type CommandMediator struct {
	commandHandler eventhorizon.CommandHandler
	repository     accountreadmodel.ReadModeler
	idCreator      eventstoredb.IdGenerator
}

func NewCommandMediator(
	commandHandler eventhorizon.CommandHandler,
	repository accountreadmodel.ReadModeler,
	idCreator eventstoredb.IdGenerator,
) *CommandMediator {
	return &CommandMediator{commandHandler: commandHandler, repository: repository, idCreator: idCreator}
}

func (mediator CommandMediator) RegisterNewAccount(
	ctx *gin.Context,
	transferObject RegisterNewAccountTransferObject,
) (*account.Id, *definitions.WalletAccountantError) {
	existingAccount, err := mediator.repository.GetByName(ctx, transferObject.Name)
	if err != nil {
		if err != nil && err != mongo.ErrNoDocuments {
			return nil, definitions.GenericError(err, nil)
		}
	}

	if existingAccount != nil {
		return nil, account.NameAlreadyExistsError(existingAccount.AccountId.String(), existingAccount.Name)
	}

	command, err := eventhorizon.CreateCommand(account.RegisterNewAccountCommand)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	registerNewAccountCommand, ok := command.(*account.RegisterNewAccount)
	if !ok {
		return nil, definitions.InvalidCommandError(account.RegisterNewAccountCommand, command.CommandType())
	}

	registerNewAccountCommand.AccountId = account.Id(mediator.idCreator.New())
	registerNewAccountCommand.BankName = account.BankName(transferObject.BankName)
	registerNewAccountCommand.Name = transferObject.Name
	registerNewAccountCommand.AccountType = common.AccountType(transferObject.AccountType)
	registerNewAccountCommand.StartingBalance = transferObject.StartingBalance
	registerNewAccountCommand.StartingBalanceDate = transferObject.StartingBalanceDate
	registerNewAccountCommand.Currency = account.Currency(transferObject.Currency)
	registerNewAccountCommand.Notes = transferObject.Notes

	err = mediator.commandHandler.HandleCommand(ctx, registerNewAccountCommand)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return &registerNewAccountCommand.AccountId, nil
}

func (mediator CommandMediator) StartNextMonth(
	ctx *gin.Context,
	accountId *account.Id,
) *definitions.WalletAccountantError {
	existingAccount, err := mediator.repository.GetByAccountId(ctx, accountId)
	if err != nil && err != mongo.ErrNoDocuments {
		return definitions.GenericError(err, nil)
	}

	if existingAccount == nil {
		return account.NonExistentAccountError(accountId.String())
	}

	command, err := eventhorizon.CreateCommand(account.StartNextMonthCommand)
	if err != nil {
		return definitions.GenericError(err, definitions.ErrorContext{"accountId": existingAccount.AccountId.String()})
	}

	startNextMonthCommand, ok := command.(*account.StartNextMonth)
	if !ok {
		return definitions.InvalidCommandError(account.StartNextMonthCommand, command.CommandType())
	}

	startNextMonthCommand.AccountId = *accountId

	err = mediator.commandHandler.HandleCommand(ctx, startNextMonthCommand)
	if err != nil {
		return definitions.GenericError(err, definitions.ErrorContext{"accountId": existingAccount.AccountId.String()})
	}

	return nil
}
