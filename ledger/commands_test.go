package ledger_test

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/commandhandler/bus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"walletaccountant/eventstoredb"
	"walletaccountant/ledger"
	"walletaccountant/mocks"
)

func setupRegisterCommandHandlerTest() map[eventhorizon.CommandType]eventhorizon.Command {
	return map[eventhorizon.CommandType]eventhorizon.Command{
		ledger.RegisterNewAccountMovementCommand: &ledger.RegisterNewAccountMovement{},
		ledger.StartAccountMonthCommand:          &ledger.StartAccountMonth{},
		ledger.EndAccountMonthCommand:            &ledger.EndAccountMonth{},
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
	requires := require.New(t)

	t.Run("successfully registers all available commands", func(t *testing.T) {
		eventStoreFactory := &eventstoredb.EventStoreFactoryMock{
			CreateEventStoreFn: func(aggregateType eventhorizon.AggregateType, batchSize uint64) eventhorizon.EventStore {
				asserts.Equal(ledger.AggregateType, aggregateType)

				return &eventstoredb.EventStoreMock{}
			},
		}

		commandHandler := bus.NewCommandHandler()

		err := ledger.RegisterCommandHandler(eventStoreFactory, commandHandler)
		requires.NoError(err)

		registeredCommands := eventhorizon.RegisteredCommands()
		asserts.Len(registeredCommands, 3)

		for expectedCommandType, expectedCommand := range availableCommands {
			asserts.Contains(registeredCommands, expectedCommandType)

			command := registeredCommands[expectedCommandType]()
			asserts.IsTypef(expectedCommand, command, string(expectedCommandType)+" type mismatch")
			asserts.Equal(expectedCommandType, command.CommandType())
			asserts.Equal(ledger.AggregateType, command.AggregateType())
		}
	})

	t.Run("fails to register all available commands, because of wrong command handler type", func(t *testing.T) {
		eventStoreFactory := &eventstoredb.EventStoreFactoryMock{
			CreateEventStoreFn: func(aggregateType eventhorizon.AggregateType, batchSize uint64) eventhorizon.EventStore {
				asserts.Equal(ledger.AggregateType, aggregateType)

				return &eventstoredb.EventStoreMock{}
			},
		}

		commandHandler := &mocks.CommandHandlerMock{}

		err := ledger.RegisterCommandHandler(eventStoreFactory, commandHandler)
		asserts.Error(err)
	})
}
