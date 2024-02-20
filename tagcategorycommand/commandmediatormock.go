package tagcategorycommand

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var _ CommandMediatorer = &CommandMediatorMock{}

type CommandMediatorMock struct {
	AddNewTagToNewCategoryFn func(
		ctx *gin.Context,
		transferObject AddNewTagToNewCategoryTransferObject,
	) (*tagcategory.TagId, *tagcategory.Id, *definitions.WalletAccountantError)

	AddNewTagToExistingCategoryFn func(
		ctx *gin.Context,
		transferObject AddNewTagToExistingCategoryTransferObject,
	) (*tagcategory.TagId, *definitions.WalletAccountantError)
}

func (mock *CommandMediatorMock) AddNewTagToNewCategory(
	ctx *gin.Context,
	transferObject AddNewTagToNewCategoryTransferObject,
) (*tagcategory.TagId, *tagcategory.Id, *definitions.WalletAccountantError) {
	if mock != nil && mock.AddNewTagToNewCategoryFn != nil {
		return mock.AddNewTagToNewCategoryFn(ctx, transferObject)
	}

	return nil, nil, nil
}

func (mock *CommandMediatorMock) AddNewTagToExistingCategory(
	ctx *gin.Context,
	transferObject AddNewTagToExistingCategoryTransferObject,
) (*tagcategory.TagId, *definitions.WalletAccountantError) {
	if mock != nil && mock.AddNewTagToExistingCategoryFn != nil {
		return mock.AddNewTagToExistingCategoryFn(ctx, transferObject)
	}

	return nil, nil
}
