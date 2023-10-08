package account

import (
	"walletaccountant/definitions"
)

const (
	NameAlreadyExistsCode definitions.ErrorCode = iota + 100
	InvalidRegisterCommandCode
	InexistentAccountCode

	GenericCode definitions.ErrorCode = 999
)

const (
	NameAlreadyExists      definitions.ErrorReason = "Account name already exists"
	InvalidRegisterCommand definitions.ErrorReason = "Invalid register command"
	InexistentAccount      definitions.ErrorReason = "Account does not exist"
)

func NameAlreadyExistsError(context definitions.ErrorContext) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NameAlreadyExistsCode,
		Reason:  NameAlreadyExists,
		Context: context,
	}
}

func InvalidRegisterCommandError(context definitions.ErrorContext) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    InvalidRegisterCommandCode,
		Reason:  InvalidRegisterCommand,
		Context: context,
	}
}

func InexistentAccountError(context definitions.ErrorContext) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    InexistentAccountCode,
		Reason:  InexistentAccount,
		Context: context,
	}
}

func GenericError(reason error, context definitions.ErrorContext) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    GenericCode,
		Reason:  definitions.ErrorReason(reason.Error()),
		Context: context,
	}
}
