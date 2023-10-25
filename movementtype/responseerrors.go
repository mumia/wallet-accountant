package movementtype

import (
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

const (
	NonexistentMovementTypeErrorCode definitions.ErrorCode = iota + 300
	NonExistentMovementTypeAccountErrorCode
	NonExistentMovementTypeSourceAccountErrorCode
	NonExistentMovementTypeTagErrorCode
	SameAccountAndSourceAccountErrorCode
)

const (
	NonExistentMovementType              definitions.ErrorReason = "Movement type does not exist"
	NonExistentMovementTypeAccount       definitions.ErrorReason = "Account for movement type does not exist"
	NonExistentMovementTypeSourceAccount definitions.ErrorReason = "Source account for movement type does not exist"
	NonExistentMovementTypeTag           definitions.ErrorReason = "Tag for movement type does not exist"
	SameAccountAndSourceAccount          definitions.ErrorReason = "Account and source account for movement type cannot be the same"
)

func NonExistentMovementTypeError(movementTypeId string) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NonexistentMovementTypeErrorCode,
		Reason:  NonExistentMovementType,
		Context: definitions.ErrorContext{"movementTypeId": movementTypeId},
	}
}

func NonExistentMovementTypeAccountError(accountId *account.Id) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NonExistentMovementTypeAccountErrorCode,
		Reason:  NonExistentMovementTypeAccount,
		Context: definitions.ErrorContext{"accountId": accountId},
	}
}

func NonExistentMovementTypeSourceAccountError(sourceAccountId *account.Id) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NonExistentMovementTypeSourceAccountErrorCode,
		Reason:  NonExistentMovementTypeSourceAccount,
		Context: definitions.ErrorContext{"sourceAccountId": sourceAccountId},
	}
}

func SameAccountAndSourceAccountError(
	accountId *account.Id,
	sourceAccountId *account.Id,
) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    SameAccountAndSourceAccountErrorCode,
		Reason:  SameAccountAndSourceAccount,
		Context: definitions.ErrorContext{"accountId": accountId, "sourceAccountId": sourceAccountId},
	}
}

func NonExistentMovementTypeTagError(tagId *tagcategory.TagId) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NonExistentMovementTypeTagErrorCode,
		Reason:  NonExistentMovementTypeTag,
		Context: definitions.ErrorContext{"tagId": tagId},
	}
}

//func NameAlreadyExistsError(tagName string) *definitions.WalletAccountantError {
//	return &definitions.WalletAccountantError{
//		Code:    NameAlreadyExistsCode,
//		Reason:  NameAlreadyExists,
//		Context: definitions.ErrorContext{"tagName": tagName},
//	}
//}
