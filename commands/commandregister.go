package commands

import (
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/commandhandler/aggregate"
	"github.com/looplab/eventhorizon/commandhandler/bus"
)

func RegisterCommands(commands []func() eventhorizon.Command) {
	for _, command := range commands {
		eventhorizon.RegisterCommand(command)
	}
}

func RegisterCommandTypes(
	aggregateStore eventhorizon.AggregateStore,
	commandBus *bus.CommandHandler,
	aggregateType eventhorizon.AggregateType,
	commandTypes []eventhorizon.CommandType,
) error {
	commandHandler, err := aggregate.NewCommandHandler(aggregateType, aggregateStore)
	if err != nil {
		return fmt.Errorf("could not create command handler. AggregateType: %s Error: %w", aggregateType, err)
	}

	for _, commandType := range commandTypes {
		if err := commandBus.SetHandler(commandHandler, commandType); err != nil {
			return fmt.Errorf("could not set command handler. AggregateType: %s Error: %w", aggregateType, err)
		}
	}

	return nil
}
