package tagcategorycommand

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategoryreadmodel"
)

var _ CommandMediatorer = &CommandMediator{}

type CommandMediatorer interface {
	AddNewTagToNewCategory(
		ctx *gin.Context,
		transferObject AddNewTagToNewCategoryTransferObject,
	) (*tagcategory.TagId, *tagcategory.Id, *definitions.WalletAccountantError)

	AddNewTagToExistingCategory(
		ctx *gin.Context,
		transferObject AddNewTagToExistingCategoryTransferObject,
	) (*tagcategory.TagId, *definitions.WalletAccountantError)
}

type CommandMediator struct {
	commandHandler eventhorizon.CommandHandler
	repository     tagcategoryreadmodel.ReadModeler
	idCreator      eventstoredb.IdGenerator
}

func NewCommandMediator(
	commandHandler eventhorizon.CommandHandler,
	repository tagcategoryreadmodel.ReadModeler,
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
) (*tagcategory.TagId, *tagcategory.Id, *definitions.WalletAccountantError) {
	responseErr := mediator.tagCategoryNameExists(ctx, transferObject.CategoryName)
	if responseErr != nil {
		return nil, nil, responseErr
	}

	responseErr = mediator.tagNameExists(ctx, transferObject.TagName)
	if responseErr != nil {
		return nil, nil, responseErr
	}

	command, err := eventhorizon.CreateCommand(tagcategory.AddNewTagToNewCategoryCommand)
	if err != nil {
		return nil, nil, definitions.GenericError(err, nil)
	}

	addNewTagToNewCategoryCommand, ok := command.(*tagcategory.AddNewTagToNewCategory)
	if !ok {
		return nil, nil, definitions.InvalidCommandError(tagcategory.AddNewTagToNewCategoryCommand, command.CommandType())
	}

	addNewTagToNewCategoryCommand.TagCategoryId = tagcategory.Id(mediator.idCreator.New())
	addNewTagToNewCategoryCommand.Name = transferObject.CategoryName
	addNewTagToNewCategoryCommand.Notes = transferObject.CategoryNotes
	addNewTagToNewCategoryCommand.Tag = tagcategory.NewTag{
		TagId: tagcategory.TagId(mediator.idCreator.New()),
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
) (*tagcategory.TagId, *definitions.WalletAccountantError) {
	tagCategoryIdUUID, err := uuid.Parse(transferObject.TagCategoryId)
	tagCategoryId := tagcategory.Id(tagCategoryIdUUID)

	responseErr := mediator.tagCategoryIdExists(ctx, &tagCategoryId)
	if responseErr != nil {
		return nil, responseErr
	}

	responseErr = mediator.tagNameExists(ctx, transferObject.TagName)
	if responseErr != nil {
		return nil, responseErr
	}

	command, err := eventhorizon.CreateCommand(tagcategory.AddNewTagToExistingCategoryCommand)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	addNewTagToExistingCategoryCommand, ok := command.(*tagcategory.AddNewTagToExistingCategory)
	if !ok {
		return nil, definitions.InvalidCommandError(tagcategory.AddNewTagToExistingCategoryCommand, command.CommandType())
	}

	addNewTagToExistingCategoryCommand.TagId = tagcategory.TagId(mediator.idCreator.New())
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
		return tagcategory.CategoryNameAlreadyExistsError(categoryName)
	}

	return nil
}

func (mediator CommandMediator) tagCategoryIdExists(
	ctx *gin.Context,
	categoryId *tagcategory.Id,
) *definitions.WalletAccountantError {
	tagCategoryExists, err := mediator.repository.CategoryExistsById(ctx, categoryId)
	if err != nil {
		return definitions.GenericError(err, nil)
	}

	if !tagCategoryExists {
		return tagcategory.NonexistentCategoryError(categoryId)
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
		return tagcategory.NameAlreadyExistsError(tagName)
	}

	return nil
}
