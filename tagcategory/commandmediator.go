package tagcategory

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
)

var _ CommandMediatorer = &CommandMediator{}

type CommandMediatorer interface {
	AddNewTagToNewCategory(
		ctx *gin.Context,
		transferObject AddNewTagToNewCategoryTransferObject,
	) (*TagId, *Id, *definitions.WalletAccountantError)

	AddNewTagToExistingCategory(
		ctx *gin.Context,
		transferObject AddNewTagToExistingCategoryTransferObject,
	) (*TagId, *definitions.WalletAccountantError)
}

type CommandMediator struct {
	commandHandler eventhorizon.CommandHandler
	repository     ReadModeler
	idCreator      eventstoredb.IdGenerator
}

func NewCommandMediator(
	commandHandler eventhorizon.CommandHandler,
	repository ReadModeler,
	idCreator eventstoredb.IdGenerator,
) *CommandMediator {
	return &CommandMediator{
		commandHandler: commandHandler,
		repository:     repository,
		idCreator:      idCreator,
	}
}

func (mediator CommandMediator) AddNewTagToNewCategory(
	ctx *gin.Context,
	transferObject AddNewTagToNewCategoryTransferObject,
) (*TagId, *Id, *definitions.WalletAccountantError) {
	responseErr := mediator.tagCategoryNameExists(ctx, transferObject.CategoryName)
	if responseErr != nil {
		return nil, nil, responseErr
	}

	responseErr = mediator.tagNameExists(ctx, transferObject.TagName)
	if responseErr != nil {
		return nil, nil, responseErr
	}

	command, err := eventhorizon.CreateCommand(AddNewTagToNewCategoryCommand)
	if err != nil {
		return nil, nil, definitions.GenericError(err, nil)
	}

	addNewTagToNewCategoryCommand, ok := command.(*AddNewTagToNewCategory)
	if !ok {
		return nil, nil, definitions.InvalidCommandError(AddNewTagToNewCategoryCommand, command.CommandType())
	}

	addNewTagToNewCategoryCommand.TagCategoryId = Id(mediator.idCreator.New())
	addNewTagToNewCategoryCommand.Name = transferObject.CategoryName
	addNewTagToNewCategoryCommand.Notes = transferObject.CategoryNotes
	addNewTagToNewCategoryCommand.Tag = NewTag{
		TagId: TagId(mediator.idCreator.New()),
		Name:  transferObject.TagName,
		Notes: transferObject.TagNotes,
	}

	err = mediator.commandHandler.HandleCommand(ctx, addNewTagToNewCategoryCommand)
	if err != nil {
		return nil, nil, definitions.GenericError(err, nil)
	}

	return &addNewTagToNewCategoryCommand.Tag.TagId, &addNewTagToNewCategoryCommand.TagCategoryId, nil
}

func (mediator CommandMediator) AddNewTagToExistingCategory(
	ctx *gin.Context,
	transferObject AddNewTagToExistingCategoryTransferObject,
) (*TagId, *definitions.WalletAccountantError) {
	tagCategoryIdUUID, err := uuid.Parse(transferObject.TagCategoryId)
	tagCategoryId := Id(tagCategoryIdUUID)

	responseErr := mediator.tagCategoryIdExists(ctx, &tagCategoryId)
	if responseErr != nil {
		return nil, responseErr
	}

	responseErr = mediator.tagNameExists(ctx, transferObject.TagName)
	if responseErr != nil {
		return nil, responseErr
	}

	command, err := eventhorizon.CreateCommand(AddNewTagToExistingCategoryCommand)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	addNewTagToExistingCategoryCommand, ok := command.(*AddNewTagToExistingCategory)
	if !ok {
		return nil, definitions.InvalidCommandError(AddNewTagToExistingCategoryCommand, command.CommandType())
	}

	addNewTagToExistingCategoryCommand.TagId = TagId(mediator.idCreator.New())
	addNewTagToExistingCategoryCommand.TagCategoryId = tagCategoryId
	addNewTagToExistingCategoryCommand.Name = transferObject.TagName
	addNewTagToExistingCategoryCommand.Notes = transferObject.TagNotes

	err = mediator.commandHandler.HandleCommand(ctx, addNewTagToExistingCategoryCommand)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return &addNewTagToExistingCategoryCommand.TagId, nil
}

func (mediator CommandMediator) tagCategoryNameExists(
	ctx *gin.Context,
	categoryName string,
) *definitions.WalletAccountantError {
	tagCategoryExists, err := mediator.repository.CategoryExistsByName(ctx, categoryName)
	if err != nil {
		return definitions.GenericError(err, nil)
	}

	if tagCategoryExists {
		return CategoryNameAlreadyExistsError(categoryName)
	}

	return nil
}

func (mediator CommandMediator) tagCategoryIdExists(
	ctx *gin.Context,
	categoryId *Id,
) *definitions.WalletAccountantError {
	tagCategoryExists, err := mediator.repository.CategoryExistsById(ctx, categoryId)
	if err != nil {
		return definitions.GenericError(err, nil)
	}

	if !tagCategoryExists {
		return NonexistentCategoryError(categoryId)
	}

	return nil
}

func (mediator CommandMediator) tagNameExists(
	ctx *gin.Context,
	tagName string,
) *definitions.WalletAccountantError {
	tagExists, err := mediator.repository.ExistsByName(ctx, tagName)
	if err != nil {
		return definitions.GenericError(err, nil)
	}

	if tagExists {
		return NameAlreadyExistsError(tagName)
	}

	return nil
}
