package movementtype

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/tagcategory"
)

var _ CommandMediatorer = &CommandMediator{}

type CommandMediatorer interface {
	RegisterNewMovementType(
		ctx *gin.Context,
		transferObject RegisterNewMovementTypeTransferObject,
	) (*Id, *definitions.WalletAccountantError)
}

type CommandMediator struct {
	commandHandler        eventhorizon.CommandHandler
	accountRepository     account.ReadModeler
	tagCategoryRepository tagcategory.ReadModeler
	idCreator             eventstoredb.IdGenerator
}

func NewCommandMediator(
	commandHandler eventhorizon.CommandHandler,
	accountRepository account.ReadModeler,
	tagCategoryRepository tagcategory.ReadModeler,
	idCreator eventstoredb.IdGenerator,
) *CommandMediator {
	return &CommandMediator{
		commandHandler:        commandHandler,
		accountRepository:     accountRepository,
		tagCategoryRepository: tagCategoryRepository,
		idCreator:             idCreator,
	}
}

func (mediator *CommandMediator) RegisterNewMovementType(
	ctx *gin.Context,
	transferObject RegisterNewMovementTypeTransferObject,
) (*Id, *definitions.WalletAccountantError) {
	accountId := account.Id(uuid.MustParse(transferObject.AccountId))
	var sourceAccountId *account.Id
	if transferObject.SourceAccountId != nil {
		srcAccId := account.Id(uuid.MustParse(*transferObject.SourceAccountId))
		sourceAccountId = &srcAccId
	}

	if transferObject.SourceAccountId != nil && transferObject.AccountId == *transferObject.SourceAccountId {
		return nil, SameAccountAndSourceAccountError(&accountId, sourceAccountId)
	}

	command, err := eventhorizon.CreateCommand(RegisterNewMovementTypeCommand)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	registerNewMovementTypeCommand, ok := command.(*RegisterNewMovementType)
	if !ok {
		return nil, definitions.InvalidCommandError(RegisterNewMovementTypeCommand, command.CommandType())
	}

	registerNewMovementTypeCommand.MovementTypeId = Id(mediator.idCreator.New())
	registerNewMovementTypeCommand.Type = Type(transferObject.Type)
	registerNewMovementTypeCommand.Description = transferObject.Description
	registerNewMovementTypeCommand.Notes = transferObject.Notes

	exists, walletAccountantError := mediator.accountExists(ctx, &accountId)
	if walletAccountantError != nil {
		return nil, walletAccountantError
	}
	if !exists {
		return nil, NonExistentMovementTypeAccountError(&accountId)
	}
	registerNewMovementTypeCommand.AccountId = accountId

	if sourceAccountId != nil {
		exists, walletAccountantError := mediator.accountExists(ctx, sourceAccountId)
		if walletAccountantError != nil {
			return nil, walletAccountantError
		}
		if !exists {
			return nil, NonExistentMovementTypeSourceAccountError(sourceAccountId)
		}
	}
	registerNewMovementTypeCommand.SourceAccountId = sourceAccountId

	for _, tagIdString := range transferObject.TagIds {
		tagId := tagcategory.TagId(uuid.MustParse(tagIdString))

		exists, walletAccountantError := mediator.tagExists(ctx, &tagId)
		if walletAccountantError != nil {
			return nil, walletAccountantError
		}
		if !exists {
			return nil, NonExistentMovementTypeTagError(&tagId)
		}

		registerNewMovementTypeCommand.TagIds = append(registerNewMovementTypeCommand.TagIds, &tagId)
	}

	err = mediator.commandHandler.HandleCommand(ctx, registerNewMovementTypeCommand)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return &registerNewMovementTypeCommand.MovementTypeId, nil
}

func (mediator *CommandMediator) accountExists(
	ctx *gin.Context,
	accountId *account.Id,
) (bool, *definitions.WalletAccountantError) {
	foundAccount, err := mediator.accountRepository.GetByAccountId(ctx, accountId)
	if err != nil && err != mongo.ErrNoDocuments {
		return false, definitions.GenericError(err, nil)
	}

	return foundAccount != nil, nil
}

func (mediator *CommandMediator) tagExists(
	ctx *gin.Context,
	tagId *tagcategory.TagId,
) (bool, *definitions.WalletAccountantError) {
	foundTag, err := mediator.tagCategoryRepository.ExistsById(ctx, tagId)
	if err != nil && err != mongo.ErrNoDocuments {
		return false, definitions.GenericError(err, nil)
	}

	return foundTag, nil
}
