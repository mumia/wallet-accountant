package definitions

import (
	"fmt"
	"github.com/looplab/eventhorizon"
	"runtime"
)

const (
	InvalidCommandCode ErrorCode = iota + 900

	GenericCode ErrorCode = 999
)

const (
	InvalidCommand ErrorReason = "Invalid command"
)

func InvalidCommandError(expected eventhorizon.CommandType, found eventhorizon.CommandType) *WalletAccountantError {
	return &WalletAccountantError{
		Code:    InvalidCommandCode,
		Reason:  InvalidCommand,
		Context: ErrorContext{"expected": expected, "found": found},
	}
}

func GenericError(reason error, context ErrorContext) *WalletAccountantError {
	if context == nil {
		context = ErrorContext{}
	}

	skip := 1
	var trace []string
	for {
		_, file, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}

		trace = append(trace, fmt.Sprintf("%s: %d", file, line))

		skip++
	}
	context["trace"] = trace

	return &WalletAccountantError{
		Code:    GenericCode,
		Reason:  ErrorReason(reason.Error()),
		Context: context,
	}
}
