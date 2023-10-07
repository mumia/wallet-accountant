package account_test

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/commandhandler/bus"
	"github.com/stretchr/testify/assert"
	"testing"
	"walletaccountant/account"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
)

func setupRegisterCommandHandlerTest() map[eventhorizon.CommandType]eventhorizon.Command {
	return map[eventhorizon.CommandType]eventhorizon.Command{
		account.RegisterNewAccountCommand: &account.RegisterNewAccount{},
		account.StartNextMonthCommand:     &account.StartNextMonth{},
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
			CreateEventStoreFn: func(aggregateType eventhorizon.AggregateType) eventhorizon.EventStore {
				asserts.Equal(account.AggregateType, aggregateType)

				return &eventstoredb.EventStoreMock{}
			},
		}

		commandHandler := bus.NewCommandHandler()

		err := account.RegisterCommandHandler(eventStoreFactory, commandHandler)
		asserts.NoError(err)

		registeredCommands := eventhorizon.RegisteredCommands()
		asserts.Len(registeredCommands, 2)

		for expectedCommandType, expectedCommand := range availableCommands {
			asserts.Contains(registeredCommands, expectedCommandType)

			command := registeredCommands[expectedCommandType]()
			asserts.IsTypef(expectedCommand, command, string(expectedCommandType)+" type mismatch")
			asserts.Equal(expectedCommandType, command.CommandType())
			asserts.Equal(account.AggregateType, command.AggregateType())
		}
	})

	t.Run("fails to register all available commands, because of wrong command handler type", func(t *testing.T) {
		eventStoreFactory := &eventstoredb.EventStoreFactoryMock{
			CreateEventStoreFn: func(aggregateType eventhorizon.AggregateType) eventhorizon.EventStore {
				asserts.Equal(account.AggregateType, aggregateType)

				return &eventstoredb.EventStoreMock{}
			},
		}

		commandHandler := &mocks.CommandHandlerMock{}

		err := account.RegisterCommandHandler(eventStoreFactory, commandHandler)
		asserts.Error(err)
	})
}
