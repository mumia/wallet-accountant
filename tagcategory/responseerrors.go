package tagcategory

import (
	"walletaccountant/definitions"
)

const (
	CategoryNameAlreadyExistsCode definitions.ErrorCode = iota + 200
	NonexistentCategoryErrorCode
	NameAlreadyExistsCode
)

const (
	CategoryNameAlreadyExists definitions.ErrorReason = "TagCategory category name already exists"
	NonexistentCategory       definitions.ErrorReason = "TagCategory does not exist"
	NameAlreadyExists         definitions.ErrorReason = "Tag name already exists"
)

func CategoryNameAlreadyExistsError(categoryName string) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    CategoryNameAlreadyExistsCode,
		Reason:  CategoryNameAlreadyExists,
		Context: definitions.ErrorContext{"tagCategoryName": categoryName},
	}
}

func NonexistentCategoryError(categoryId *Id) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NonexistentCategoryErrorCode,
		Reason:  NonexistentCategory,
		Context: definitions.ErrorContext{"tagCategoryId": categoryId},
	}
}

func NameAlreadyExistsError(tagName string) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NameAlreadyExistsCode,
		Reason:  NameAlreadyExists,
		Context: definitions.ErrorContext{"tagName": tagName},
	}
}
