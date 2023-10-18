package account

import (
	"walletaccountant/definitions"
)

const (
	NameAlreadyExistsCode definitions.ErrorCode = iota + 100
	NonExistentAccountCode
)

const (
	NameAlreadyExists  definitions.ErrorReason = "Account name already exists"
	NonExistentAccount definitions.ErrorReason = "Account does not exist"
)

func NameAlreadyExistsError(existingAccountId string, existingAccountName string) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:   NameAlreadyExistsCode,
		Reason: NameAlreadyExists,
		Context: definitions.ErrorContext{
			"existingAccountId":   existingAccountId,
			"existingAccountName": existingAccountName,
		},
	}
}

func NonExistentAccountError(accountId string) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NonExistentAccountCode,
		Reason:  NonExistentAccount,
		Context: definitions.ErrorContext{"AccountId": accountId},
	}
}
