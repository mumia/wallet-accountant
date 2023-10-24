package movementtype_test

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/commandhandler/bus"
	"github.com/stretchr/testify/assert"
	"testing"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
	"walletaccountant/movementtype"
)

func setupRegisterCommandHandlerTest() map[eventhorizon.CommandType]eventhorizon.Command {
	return map[eventhorizon.CommandType]eventhorizon.Command{
		movementtype.RegisterNewMovementTypeCommand: &movementtype.RegisterNewMovementType{},
	}
}

func tearDownRegisterCommandHandlerTest(commandTypes map[eventhorizon.CommandType]eventhorizon.Command) {
	for commandType, _ := range commandTypes {
		eventhorizon.UnregisterCommand(commandType)
	}
}

func TestRegisterCommandHandler(t *testing.T) {
	availableCommands := setupRegisterCommandHandlerTest()
	defer tearDownRegisterCommandHandlerTest(availableCommands)

	asserts := assert.New(t)

	t.Run("successfully registers all available commands", func(t *testing.T) {
		eventStoreFactory := &eventstoredb.EventStoreFactoryMock{
			CreateEventStoreFn: func(aggregateType eventhorizon.AggregateType, batchSize uint64) eventhorizon.EventStore {
				asserts.Equal(movementtype.AggregateType, aggregateType)

				return &eventstoredb.EventStoreMock{}
			},
		}

		commandHandler := bus.NewCommandHandler()

		err := movementtype.RegisterCommandHandler(eventStoreFactory, commandHandler)
		asserts.NoError(err)

		registeredCommands := eventhorizon.RegisteredCommands()
		asserts.Len(registeredCommands, len(availableCommands))

		for expectedCommandType, expectedCommand := range availableCommands {
			asserts.Contains(registeredCommands, expectedCommandType)

			command := registeredCommands[expectedCommandType]()
			asserts.IsTypef(expectedCommand, command, string(expectedCommandType)+" type mismatch")
			asserts.Equal(expectedCommandType, command.CommandType())
			asserts.Equal(movementtype.AggregateType, command.AggregateType())
		}
	})

	t.Run("fails to register all available commands, because of wrong command handler type", func(t *testing.T) {
		eventStoreFactory := &eventstoredb.EventStoreFactoryMock{
			CreateEventStoreFn: func(aggregateType eventhorizon.AggregateType, batchSize uint64) eventhorizon.EventStore {
				asserts.Equal(movementtype.AggregateType, aggregateType)

				return &eventstoredb.EventStoreMock{}
			},
		}

		commandHandler := &mocks.CommandHandlerMock{}

		err := movementtype.RegisterCommandHandler(eventStoreFactory, commandHandler)
		asserts.Error(err)
	})
}
