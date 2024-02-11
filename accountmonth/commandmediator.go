package accountmonth

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
	"walletaccountant/account"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

var _ CommandMediatorer = &CommandMediator{}

type CommandMediatorer interface {
	RegisterNewAccountMovement(
		ctx *gin.Context,
		transferObject RegisterNewAccountMovementTransferObject,
	) *definitions.WalletAccountantError
	EndAccountMonth(ctx *gin.Context, transferObject EndAccountMonthTransferObject) *definitions.WalletAccountantError
}

type CommandMediator struct {
	commandHandler         eventhorizon.CommandHandler
	repository             ReadModeler
	accountRepository      account.ReadModeler
	movementTypeRepository movementtype.ReadModeler
	idCreator              eventstoredb.IdGenerator
}

func NewCommandMediator(
	commandHandler eventhorizon.CommandHandler,
	repository ReadModeler,
	accountRepository account.ReadModeler,
	movementTypeRepository movementtype.ReadModeler,
	idCreator eventstoredb.IdGenerator,
) *CommandMediator {
	return &CommandMediator{
		commandHandler:         commandHandler,
		repository:             repository,
		accountRepository:      accountRepository,
		movementTypeRepository: movementTypeRepository,
		idCreator:              idCreator,
	}
}

func (mediator CommandMediator) RegisterNewAccountMovement(
	ctx *gin.Context,
	transferObject RegisterNewAccountMovementTransferObject,
) *definitions.WalletAccountantError {
	accountId := account.Id(uuid.MustParse(transferObject.AccountId))
	month := transferObject.Date.Month()
	year := uint(transferObject.Date.Year())

	var movementTypeId = new(movementtype.Id)
	if transferObject.MovementTypeId != nil {
		parsedId := movementtype.Id(uuid.MustParse(*transferObject.MovementTypeId))
		movementTypeId = &parsedId
	}

	foundAccount, waErr := mediator.validateAccount(ctx, &accountId, movementTypeId, month, year)
	if waErr != nil {
		return waErr
	}

	movementType, waErr := mediator.validateMovementType(ctx, movementTypeId, transferObject)
	if waErr != nil {
		return waErr
	}

	waErr = mediator.validateMovementTypeAccountMatch(movementType, foundAccount, transferObject)
	if waErr != nil {
		return waErr
	}

	command, err := eventhorizon.CreateCommand(RegisterNewAccountMovementCommand)
	if err != nil {
		return definitions.GenericError(err, nil)
	}

	registerNewAccountMovementCommand, ok := command.(*RegisterNewAccountMovement)
	if !ok {
		return definitions.InvalidCommandError(RegisterNewAccountMovementCommand, command.CommandType())
	}

	accountMonthId, err := GenerateAccountMonthId(
		&accountId,
		transferObject.Date.Month(),
		uint(transferObject.Date.Year()),
	)

	registerNewAccountMovementCommand.AccountMonthId = *accountMonthId
	registerNewAccountMovementCommand.AccountMovementId = AccountMovementId(mediator.idCreator.New())
	if movementType != nil {
		registerNewAccountMovementCommand.MovementTypeId = movementType.MovementTypeId
	}
	registerNewAccountMovementCommand.Action = common.MovementActionBuilder(transferObject.Action)
	registerNewAccountMovementCommand.Amount = transferObject.Amount
	registerNewAccountMovementCommand.Date = transferObject.Date
	if transferObject.SourceAccountId != nil {
		registerNewAccountMovementCommand.SourceAccountId = account.IdBuilder(
			uuid.MustParse(*transferObject.SourceAccountId),
		)
	}
	registerNewAccountMovementCommand.Description = transferObject.Description
	registerNewAccountMovementCommand.Notes = transferObject.Notes
	registerNewAccountMovementCommand.TagIds = tagcategory.TagIdsFromStrings(transferObject.TagIds)

	err = mediator.commandHandler.HandleCommand(ctx, registerNewAccountMovementCommand)
	if err != nil {
		return definitions.GenericError(err, nil)
	}

	return nil
}

func (mediator CommandMediator) EndAccountMonth(
	ctx *gin.Context,
	transferObject EndAccountMonthTransferObject,
) *definitions.WalletAccountantError {
	accountId := account.Id(uuid.MustParse(transferObject.AccountId))

	foundAccount, waErr := mediator.validateAccount(
		ctx,
		&accountId,
		nil,
		transferObject.Month,
		transferObject.Year,
	)
	if waErr != nil {
		return waErr
	}

	accountMonth, err := mediator.repository.GetByAccountActiveMonth(ctx, foundAccount)
	if err != nil && err != mongo.ErrNoDocuments {
		return definitions.GenericError(err, nil)
	}

	if accountMonth == nil {
		return NonExistentAccountMonthError(
			transferObject.AccountId,
			"",
			int(transferObject.Month),
			int(transferObject.Year),
		)
	}

	if accountMonth.MonthEnded {
		return AlreadyEndedError(
			accountMonth.AccountMonthId.String(),
			foundAccount.AccountId.String(),
			int(accountMonth.ActiveMonth.Month),
			int(accountMonth.ActiveMonth.Year),
		)
	}

	if accountMonth.Balance != *transferObject.EndBalance {
		return MismatchedEndBalanceError(
			accountMonth.AccountMonthId.String(),
			accountMonth.Balance,
			*transferObject.EndBalance,
			int(accountMonth.ActiveMonth.Month),
			int(accountMonth.ActiveMonth.Year),
		)
	}

	command, err := eventhorizon.CreateCommand(EndAccountMonthCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{"accountMonthId": accountMonth.AccountMonthId.String()},
		)
	}

	endAccountMonthCommand, ok := command.(*EndAccountMonth)
	if !ok {
		return definitions.InvalidCommandError(EndAccountMonthCommand, command.CommandType())
	}

	endAccountMonthCommand.AccountMonthId = *accountMonth.AccountMonthId
	endAccountMonthCommand.AccountId = accountId
	endAccountMonthCommand.EndBalance = *transferObject.EndBalance
	endAccountMonthCommand.Month = transferObject.Month
	endAccountMonthCommand.Year = transferObject.Year

	err = mediator.commandHandler.HandleCommand(ctx, endAccountMonthCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{"accountMonthId": accountMonth.AccountMonthId.String()},
		)
	}

	return nil
}

func (mediator CommandMediator) validateAccount(
	ctx context.Context,
	accountId *account.Id,
	movementTypeId *movementtype.Id,
	month time.Month,
	year uint,
) (*account.Entity, *definitions.WalletAccountantError) {
	foundAccount, err := mediator.accountRepository.GetByAccountId(ctx, accountId)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, definitions.GenericError(err, definitions.ErrorContext{"accountId": accountId})
	}

	if foundAccount == nil {
		return nil, NonExistentAccountError(accountId.String(), int(month), int(year))
	}

	if month != foundAccount.ActiveMonth.Month || year != foundAccount.ActiveMonth.Year {
		movementTypeIdString := ""
		if movementTypeId != nil {
			movementTypeIdString = movementTypeId.String()
		}

		return nil, MismatchedActiveMonthError(
			accountId.String(),
			movementTypeIdString,
			int(foundAccount.ActiveMonth.Month),
			int(foundAccount.ActiveMonth.Year),
			int(month),
			int(year),
		)
	}

	return foundAccount, nil
}

func (mediator CommandMediator) validateMovementType(
	ctx context.Context,
	movementTypeId *movementtype.Id,
	transferObject RegisterNewAccountMovementTransferObject,
) (*movementtype.Entity, *definitions.WalletAccountantError) {
	month := transferObject.Date.Month()
	year := uint(transferObject.Date.Year())

	if transferObject.MovementTypeId == nil {
		return nil, nil
	}

	movementType, err := mediator.movementTypeRepository.GetByMovementTypeId(ctx, movementTypeId)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, definitions.GenericError(err, definitions.ErrorContext{"movementTypeId": transferObject.MovementTypeId})
	}

	if movementType == nil {
		return nil, NonExistentMovementTypeError(
			transferObject.AccountId,
			transferObject.MovementTypeId,
			int(month),
			int(year),
		)
	}

	return movementType, nil
}

func (mediator CommandMediator) validateMovementTypeAccountMatch(
	movementType *movementtype.Entity,
	foundAccount *account.Entity,
	transferObject RegisterNewAccountMovementTransferObject,
) *definitions.WalletAccountantError {
	if movementType == nil {
		return nil
	}

	if movementType.AccountId.String() != foundAccount.AccountId.String() {
		return MismatchedAccountIdError(
			foundAccount.AccountId.String(),
			movementType.MovementTypeId.String(),
			int(transferObject.Date.Month()),
			transferObject.Date.Year(),
		)
	}

	return nil
}
