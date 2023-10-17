package tagcategory

import (
	"github.com/looplab/eventhorizon"
	"runtime"
	"walletaccountant/definitions"
)

const (
	CategoryNameAlreadyExistsCode definitions.ErrorCode = iota + 200
	NonexistentCategoryErrorCode
	NameAlreadyExistsCode
	InvalidCommandCode
	//InexistentAccountCode

	GenericCode definitions.ErrorCode = 999
)

const (
	CategoryNameAlreadyExists definitions.ErrorReason = "TagCategory category name already exists"
	NonexistentCategory       definitions.ErrorReason = "TagCategory does not exist"
	NameAlreadyExists         definitions.ErrorReason = "Tag name already exists"
	InvalidCommand            definitions.ErrorReason = "Invalid command"
	//InexistentAccount      definitions.ErrorReason = "Account does not exist"
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

func InvalidCommandError(
	expected eventhorizon.CommandType,
	found eventhorizon.CommandType,
) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    InvalidCommandCode,
		Reason:  InvalidCommand,
		Context: definitions.ErrorContext{"expected": expected, "found": found},
	}
}

func GenericError(reason error, context definitions.ErrorContext) *definitions.WalletAccountantError {
	if context == nil {
		context = definitions.ErrorContext{}
	}

	skip := 1
	for {
		_, file, line, ok := runtime.Caller(skip)

		if !ok {
			break
		}

		context[file] = line

		skip++
	}

	return &definitions.WalletAccountantError{
		Code:    GenericCode,
		Reason:  definitions.ErrorReason(reason.Error()),
		Context: context,
	}
}
