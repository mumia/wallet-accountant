package account

import "encoding/json"

type ErrorCode int
type ErrorReason string
type ErrorContext map[string]any

const (
	NameAlreadyExistsCode ErrorCode = iota + 100
	InvalidRegisterCommandCode
	InexistentAccountCode

	GenericCode ErrorCode = 999
)

const (
	NameAlreadyExists      ErrorReason = "Account name already exists"
	InvalidRegisterCommand ErrorReason = "Invalid register command"
	InexistentAccount      ErrorReason = "Account does not exist"
)

type ErrorResponse struct {
	Code    ErrorCode    `json:"code"`
	Reason  ErrorReason  `json:"reason"`
	Context ErrorContext `json:"context"`
}

func (error ErrorResponse) Error() string {
	response, err := json.Marshal(error)
	if err != nil {
		panic(err)
	}

	return string(response)
}

func NameAlreadyExistsError(context ErrorContext) error {
	return ErrorResponse{
		Code:    NameAlreadyExistsCode,
		Reason:  NameAlreadyExists,
		Context: context,
	}
}

func InvalidRegisterCommandError(context ErrorContext) error {
	return ErrorResponse{
		Code:    InvalidRegisterCommandCode,
		Reason:  InvalidRegisterCommand,
		Context: context,
	}
}

func InexistentAccountError(context ErrorContext) error {
	return ErrorResponse{
		Code:    InexistentAccountCode,
		Reason:  InexistentAccount,
		Context: context,
	}
}

func GenericError(reason error, context ErrorContext) error {
	return ErrorResponse{
		Code:    GenericCode,
		Reason:  ErrorReason(reason.Error()),
		Context: context,
	}
}
