package account

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"walletaccountant/commands"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
)

var newId = uuid.New()
var expectedAccountId = Id(newId)
var bankName = "my bank name"
var name = "account name"
var accountType = Type(Checking)
var startingBalance = float64(1269)
var startingBalanceDate = time.Now()
var currency = Currency(USD)
var notes = "my account notes"

func setupCommandMediatorTest() {
	commands.RegisterCommands(
		[]func() eventhorizon.Command{
			func() eventhorizon.Command { return &RegisterNewAccount{} },
			func() eventhorizon.Command { return &StartNextMonth{} },
		},
	)
}

func tearDownCommandMediatorTest() {
	eventhorizon.UnregisterCommand(RegisterNewAccountCommand)
	eventhorizon.UnregisterCommand(StartNextMonthCommand)
}

func TestCommandMediator_RegisterNewAccount(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)

	transferObject := RegisterNewAccountTransferObject{
		BankName:            bankName,
		Name:                name,
		AccountType:         int(accountType),
		StartingBalance:     startingBalance,
		StartingBalanceDate: startingBalanceDate,
		Currency:            string(currency),
		Notes:               notes,
	}

	t.Run("correctly handles register new account", func(t *testing.T) {
		expectedCommand := &RegisterNewAccount{
			AccountId:           expectedAccountId,
			BankName:            bankName,
			Name:                name,
			AccountType:         accountType,
			StartingBalance:     startingBalance,
			StartingBalanceDate: startingBalanceDate,
			Currency:            currency,
			Notes:               notes,
		}

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				asserts.Equal(expectedCommand, command)

				return nil
			},
		}
		readModelRepository := &ReadModelRepositoryMock{
			GetByNameFn: func(ctx context.Context, name string) (*Entity, error) {
				return nil, nil
			},
		}

		idCreator := &eventstoredb.IdCreatorMock{
			NewFn: func() uuid.UUID {
				return newId
			},
		}

		commandMediator := NewCommandMediator(commandHandler, readModelRepository, idCreator)

		accountId, err := commandMediator.RegisterNewAccount(&gin.Context{}, transferObject)
		asserts.NoError(err)

		asserts.Equal(&expectedAccountId, accountId)
	})

	failureTestCases := [...]struct {
		testName            string
		commandHandler      *mocks.CommandHandlerMock
		readModelRepository *ReadModelRepositoryMock
		idCreator           *eventstoredb.IdCreatorMock
	}{
		{
			"fails to handle register new account, because account name already exists",
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					asserts.Fail("handle command should not be called on a failure")

					return nil
				},
			},
			&ReadModelRepositoryMock{
				GetByNameFn: func(ctx context.Context, name string) (*Entity, error) {
					return &Entity{
						AccountId:           &expectedAccountId,
						BankName:            bankName,
						Name:                name,
						AccountType:         accountType,
						StartingBalance:     startingBalance,
						StartingBalanceDate: startingBalanceDate,
						Currency:            currency,
						Notes:               notes,
						ActiveMonth:         EntityActiveMonth{},
					}, nil
				},
			},
			&eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					asserts.Fail("id creator should not be called on a failure")

					return uuid.New()
				},
			},
		},
		{
			"fails to handle register new account, because of err on command handler",
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					asserts.Equal(expectedAccountId, command.AggregateID())

					return fmt.Errorf("an error")
				},
			},
			&ReadModelRepositoryMock{
				GetByNameFn: func(ctx context.Context, name string) (*Entity, error) {
					return nil, nil
				},
			},
			&eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return newId
				},
			},
		},
	}

	for _, testCase := range failureTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			commandMediator := NewCommandMediator(
				testCase.commandHandler,
				testCase.readModelRepository,
				testCase.idCreator,
			)

			accountId, err := commandMediator.RegisterNewAccount(&gin.Context{}, transferObject)
			asserts.Error(err)
			asserts.Nil(accountId)
		})
	}
}

func TestCommandMediator_StartNextMonth(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)

	t.Run("successfully starts next month on account", func(t *testing.T) {
		expectedCommand := &StartNextMonth{AccountId: expectedAccountId}

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				asserts.Equal(expectedCommand, command)

				return nil
			},
		}

		readModelRepository := &ReadModelRepositoryMock{
			GetByAccountIdFn: func(ctx context.Context, accountId *Id) (*Entity, error) {
				asserts.Equal(&expectedAccountId, accountId)

				return &Entity{
					AccountId:           accountId,
					BankName:            bankName,
					Name:                name,
					AccountType:         accountType,
					StartingBalance:     startingBalance,
					StartingBalanceDate: startingBalanceDate,
					Currency:            currency,
					Notes:               notes,
					ActiveMonth:         EntityActiveMonth{},
				}, nil
			},
		}

		commandMediator := NewCommandMediator(commandHandler, readModelRepository, nil)

		err := commandMediator.StartNextMonth(&gin.Context{}, &expectedAccountId)
		asserts.NoError(err)
	})

	failureTestCases := [...]struct {
		testName            string
		commandHandler      *mocks.CommandHandlerMock
		readModelRepository *ReadModelRepositoryMock
		idCreator           *eventstoredb.IdCreatorMock
	}{
		{
			"fails to handle start next month, because account does not exist",
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					asserts.Fail("handle command should not be called when account doesn't exist")

					return nil
				},
			},
			&ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *Id) (*Entity, error) {
					asserts.Equal(&expectedAccountId, accountId)

					return nil, nil
				},
			},
			&eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					asserts.Fail("id creator should not be called when account doesn't exist")

					return uuid.New()
				},
			},
		},
		{
			"fails to handle start next month, because of err on command handler",
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					asserts.Equal(expectedAccountId, command.AggregateID())

					return fmt.Errorf("an error")
				},
			},
			&ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *Id) (*Entity, error) {
					asserts.Equal(&expectedAccountId, accountId)

					return &Entity{
						AccountId:           accountId,
						BankName:            bankName,
						Name:                name,
						AccountType:         accountType,
						StartingBalance:     startingBalance,
						StartingBalanceDate: startingBalanceDate,
						Currency:            currency,
						Notes:               notes,
						ActiveMonth:         EntityActiveMonth{},
					}, nil
				},
			},
			&eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return newId
				},
			},
		},
	}

	for _, testCase := range failureTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			commandMediator := NewCommandMediator(
				testCase.commandHandler,
				testCase.readModelRepository,
				testCase.idCreator,
			)

			err := commandMediator.StartNextMonth(&gin.Context{}, &expectedAccountId)
			asserts.Error(err)
		})
	}
}
