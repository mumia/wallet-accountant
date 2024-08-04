package ledger

import (
	"walletaccountant/definitions"
)

const (
	NonExistentAccountErrorCode definitions.ErrorCode = iota + 400
	NonExistentMovementTypeErrorCode
	MismatchedAccountIdErrorCode
	MismatchedActiveMonthErrorCode
	NonExistentAccountMonthErrorCode
	AlreadyEndedErrorCode
	MismatchedEndBalanceErrorCode
	NonExistentAccountIdErrorCode
)

const (
	NonExistentAccount      definitions.ErrorReason = "Account for account month does not exist"
	NonExistentMovementType definitions.ErrorReason = "Movement type for account movement does not exist"
	MismatchedAccountId     definitions.ErrorReason = "Movement type and account have different ids"
	MismatchedActiveMonth   definitions.ErrorReason = "Active month is different"
	NonExistentAccountMonth definitions.ErrorReason = "Account month does not exist"
	AlreadyEnded            definitions.ErrorReason = "Account month already ended"
	MismatchedEndBalance    definitions.ErrorReason = "Account month does not match end balance"
	NonExistentAccountId    definitions.ErrorReason = "Account id for account month does not exist"
)

func NonExistentAccountError(
	accountId string,
	month int,
	year int,
) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:   NonExistentAccountErrorCode,
		Reason: NonExistentAccount,
		Context: definitions.ErrorContext{
			"accountId": accountId,
			"month":     month,
			"year":      year,
		},
	}
}

func NonExistentMovementTypeError(
	accountId string,
	movementTypeId *string,
	month int,
	year int,
) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:   NonExistentMovementTypeErrorCode,
		Reason: NonExistentMovementType,
		Context: definitions.ErrorContext{
			"accountId":      accountId,
			"movementTypeId": movementTypeId,
			"month":          month,
			"year":           year,
		},
	}
}

func MismatchedAccountIdError(
	accountId string,
	movementTypeId string,
	month int,
	year int,
) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:   MismatchedAccountIdErrorCode,
		Reason: MismatchedAccountId,
		Context: definitions.ErrorContext{
			"accountId":      accountId,
			"movementTypeId": movementTypeId,
			"month":          month,
			"year":           year,
		},
	}
}

func MismatchedActiveMonthError(
	accountId string,
	movementTypeId string,
	accountMonth int,
	accountYear int,
	month int,
	year int,
) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:   MismatchedActiveMonthErrorCode,
		Reason: MismatchedActiveMonth,
		Context: definitions.ErrorContext{
			"accountId":      accountId,
			"movementTypeId": movementTypeId,
			"accountMonth":   accountMonth,
			"accountYear":    accountYear,
			"month":          month,
			"year":           year,
		},
	}
}

func NonExistentAccountMonthError(
	accountId string,
	movementTypeId string,
	month int,
	year int,
) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:   NonExistentAccountMonthErrorCode,
		Reason: NonExistentAccountMonth,
		Context: definitions.ErrorContext{
			"accountId":      accountId,
			"movementTypeId": movementTypeId,
			"month":          month,
			"year":           year,
		},
	}
}

func AlreadyEndedError(
	accountMonthId string,
	accountId string,
	month int,
	year int,
) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:   AlreadyEndedErrorCode,
		Reason: AlreadyEnded,
		Context: definitions.ErrorContext{
			"accountMonthId": accountMonthId,
			"accountId":      accountId,
			"month":          month,
			"year":           year,
		},
	}
}

func MismatchedEndBalanceError(
	accountMonthId string,
	accountMonthBalance int64,
	endMonthBalance int64,
	month int,
	year int,
) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:   MismatchedEndBalanceErrorCode,
		Reason: MismatchedEndBalance,
		Context: definitions.ErrorContext{
			"accountMonthId":      accountMonthId,
			"accountMonthBalance": accountMonthBalance,
			"endMonthBalance":     endMonthBalance,
			"month":               month,
			"year":                year,
		},
	}
}

func NonExistentAccountIdError(accountId string) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NonExistentAccountIdErrorCode,
		Reason:  NonExistentAccountId,
		Context: definitions.ErrorContext{"accountId": accountId},
	}
}
