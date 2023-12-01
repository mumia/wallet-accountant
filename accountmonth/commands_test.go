package accountmonth_test

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/commandhandler/bus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"walletaccountant/accountmonth"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
)

func setupRegisterCommandHandlerTest() map[eventhorizon.CommandType]eventhorizon.Command {
	return map[eventhorizon.CommandType]eventhorizon.Command{
		accountmonth.RegisterNewAccountMovementCommand: &accountmonth.RegisterNewAccountMovement{},
		accountmonth.StartAccountMonthCommand:          &accountmonth.StartAccountMonth{},
		accountmonth.EndAccountMonthCommand:            &accountmonth.EndAccountMonth{},
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
				asserts.Equal(accountmonth.AggregateType, aggregateType)

				return &eventstoredb.EventStoreMock{}
			},
		}

		commandHandler := bus.NewCommandHandler()

		err := accountmonth.RegisterCommandHandler(eventStoreFactory, commandHandler)
		requires.NoError(err)

		registeredCommands := eventhorizon.RegisteredCommands()
		asserts.Len(registeredCommands, 3)

		for expectedCommandType, expectedCommand := range availableCommands {
			asserts.Contains(registeredCommands, expectedCommandType)

			command := registeredCommands[expectedCommandType]()
			asserts.IsTypef(expectedCommand, command, string(expectedCommandType)+" type mismatch")
			asserts.Equal(expectedCommandType, command.CommandType())
			asserts.Equal(accountmonth.AggregateType, command.AggregateType())
		}
	})

	t.Run("fails to register all available commands, because of wrong command handler type", func(t *testing.T) {
		eventStoreFactory := &eventstoredb.EventStoreFactoryMock{
			CreateEventStoreFn: func(aggregateType eventhorizon.AggregateType, batchSize uint64) eventhorizon.EventStore {
				asserts.Equal(accountmonth.AggregateType, aggregateType)

				return &eventstoredb.EventStoreMock{}
			},
		}

		commandHandler := &mocks.CommandHandlerMock{}

		err := accountmonth.RegisterCommandHandler(eventStoreFactory, commandHandler)
		asserts.Error(err)
	})
}
