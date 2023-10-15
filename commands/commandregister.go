package commands

import (
	"errors"
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/commandhandler/aggregate"
	"github.com/looplab/eventhorizon/commandhandler/bus"
	"walletaccountant/eventstoredb"
)

type CommandAndType struct {
	Command     eventhorizon.Command
	CommandType eventhorizon.CommandType
}

func RegisterCommandTypes(
	eventStoreFactory eventstoredb.EventStoreCreator,
	commandHandlerBus eventhorizon.CommandHandler,
	aggregateType eventhorizon.AggregateType,
	commandAndTypes []CommandAndType,
) error {
	busCommandHandler, ok := commandHandlerBus.(*bus.CommandHandler)
	if !ok {
		return errors.New("command handle is not of type bus.CommandHandler")
	}

	eventStore := eventStoreFactory.CreateEventStore(aggregateType, 100)
	aggregateStore, err := events.NewAggregateStore(eventStore)
	if err != nil {
		return err
	}

	commandHandler, err := aggregate.NewCommandHandler(aggregateType, aggregateStore)
	if err != nil {
		return fmt.Errorf("could not create command handler. AggregateType: %s Error: %w", aggregateType, err)
	}

	for _, commandAndType := range commandAndTypes {
		command := commandAndType.Command
		eventhorizon.RegisterCommand(func() eventhorizon.Command { return command })

		if err := busCommandHandler.SetHandler(commandHandler, commandAndType.CommandType); err != nil {
			return fmt.Errorf("could not set command handler. AggregateType: %s Error: %w", aggregateType, err)
		}
	}

	return nil
}
