package mocks

import (
	"context"
	"github.com/looplab/eventhorizon"
)

var _ eventhorizon.CommandHandler = &CommandHandlerMock{}

type CommandHandlerMock struct {
	HandleCommandFn func(context.Context, eventhorizon.Command) error
}

func (handleMock *CommandHandlerMock) HandleCommand(ctx context.Context, command eventhorizon.Command) error {
	if handleMock != nil && handleMock.HandleCommandFn != nil {
		return handleMock.HandleCommandFn(ctx, command)
	}

	return nil
}
