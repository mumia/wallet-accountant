package ledgercommand

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/ledger"
	"walletaccountant/ledgerreadmodel"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypereadmodel"
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
	repository             ledgerreadmodel.ReadModeler
	accountRepository      accountreadmodel.ReadModeler
	movementTypeRepository movementtypereadmodel.ReadModeler
	idCreator              eventstoredb.IdGenerator
}

func NewCommandMediator(
	commandHandler eventhorizon.CommandHandler,
	repository ledgerreadmodel.ReadModeler,
	accountRepository accountreadmodel.ReadModeler,
	movementTypeRepository movementtypereadmodel.ReadModeler,
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
	accountId := account.IdFromUUIDString(transferObject.AccountId)
	month := transferObject.Date.Month()
	year := uint(transferObject.Date.Year())

	var movementTypeId = new(movementtype.Id)
	if transferObject.MovementTypeId != nil {
		movementTypeId = movementtype.IdFromUUIDString(*transferObject.MovementTypeId)
	}

	foundAccount, waErr := mediator.validateAccount(ctx, accountId, movementTypeId, month, year)
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

	command, err := eventhorizon.CreateCommand(ledger.RegisterNewAccountMovementCommand)
	if err != nil {
		return definitions.GenericError(err, nil)
	}

	registerNewAccountMovementCommand, ok := command.(*ledger.RegisterNewAccountMovement)
	if !ok {
		return definitions.InvalidCommandError(ledger.RegisterNewAccountMovementCommand, command.CommandType())
	}

	accountMonthId, err := ledger.IdGenerate(
		accountId,
		transferObject.Date.Month(),
		uint(transferObject.Date.Year()),
	)

	registerNewAccountMovementCommand.AccountMonthId = *accountMonthId
	registerNewAccountMovementCommand.AccountMovementId = *ledger.AccountMovementIdFromUUID(
		mediator.idCreator.New(),
	)
	if movementType != nil {
		registerNewAccountMovementCommand.MovementTypeId = movementType.MovementTypeId
	}
	registerNewAccountMovementCommand.Action = common.MovementActionBuilder(transferObject.Action)
	registerNewAccountMovementCommand.Amount = transferObject.Amount
	registerNewAccountMovementCommand.Date = transferObject.Date
	if transferObject.SourceAccountId != nil {
		registerNewAccountMovementCommand.SourceAccountId = account.IdFromUUID(
			uuid.MustParse(*transferObject.SourceAccountId),
		)
	}
	registerNewAccountMovementCommand.Description = transferObject.Description
	registerNewAccountMovementCommand.Notes = transferObject.Notes
	registerNewAccountMovementCommand.TagIds = tagcategory.TagIdsFromUUIDStrings(transferObject.TagIds)

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
	accountId := account.IdFromUUIDString(transferObject.AccountId)

	foundAccount, waErr := mediator.validateAccount(
		ctx,
		accountId,
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
		return ledger.NonExistentAccountMonthError(
			transferObject.AccountId,
			"",
			int(transferObject.Month),
			int(transferObject.Year),
		)
	}

	if accountMonth.MonthEnded {
		return ledger.AlreadyEndedError(
			accountMonth.AccountMonthId.String(),
			foundAccount.AccountId.String(),
			int(accountMonth.ActiveMonth.Month),
			int(accountMonth.ActiveMonth.Year),
		)
	}

	endBalance := *transferObject.EndBalance
	if accountMonth.Balance != endBalance {
		return ledger.MismatchedEndBalanceError(
			accountMonth.AccountMonthId.String(),
			accountMonth.Balance,
			endBalance,
			int(accountMonth.ActiveMonth.Month),
			int(accountMonth.ActiveMonth.Year),
		)
	}

	command, err := eventhorizon.CreateCommand(ledger.EndAccountMonthCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{"accountMonthId": accountMonth.AccountMonthId.String()},
		)
	}

	endAccountMonthCommand, ok := command.(*ledger.EndAccountMonth)
	if !ok {
		return definitions.InvalidCommandError(ledger.EndAccountMonthCommand, command.CommandType())
	}

	endAccountMonthCommand.AccountMonthId = *accountMonth.AccountMonthId
	endAccountMonthCommand.AccountId = *accountId
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
) (*accountreadmodel.Entity, *definitions.WalletAccountantError) {
	foundAccount, err := mediator.accountRepository.GetByAccountId(ctx, accountId)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, definitions.GenericError(err, definitions.ErrorContext{"accountId": accountId})
	}

	if foundAccount == nil {
		return nil, ledger.NonExistentAccountError(accountId.String(), int(month), int(year))
	}

	if month != foundAccount.ActiveMonth.Month || year != foundAccount.ActiveMonth.Year {
		movementTypeIdString := ""
		if movementTypeId != nil {
			movementTypeIdString = movementTypeId.String()
		}

		return nil, ledger.MismatchedActiveMonthError(
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
) (*movementtypereadmodel.Entity, *definitions.WalletAccountantError) {
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
		return nil, ledger.NonExistentMovementTypeError(
			transferObject.AccountId,
			transferObject.MovementTypeId,
			int(month),
			int(year),
		)
	}

	return movementType, nil
}

func (mediator CommandMediator) validateMovementTypeAccountMatch(
	movementType *movementtypereadmodel.Entity,
	foundAccount *accountreadmodel.Entity,
	transferObject RegisterNewAccountMovementTransferObject,
) *definitions.WalletAccountantError {
	if movementType == nil {
		return nil
	}

	if movementType.AccountId.String() != foundAccount.AccountId.String() {
		return ledger.MismatchedAccountIdError(
			foundAccount.AccountId.String(),
			movementType.MovementTypeId.String(),
			int(transferObject.Date.Month()),
			transferObject.Date.Year(),
		)
	}

	return nil
}
