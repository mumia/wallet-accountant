package accountcommand

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
)

var newId = uuid.New()
var expectedAccountId = account.IdFromUUID(newId)
var bankName = account.BankName("my bank name")
var name = "account name"
var accountType = common.Checking
var startingBalance int64 = 126900
var startingBalanceDate = time.Now()
var currency = account.Currency(account.USD)
var notes = "my account notes"

func setupCommandMediatorTest() {
	commands := []func() eventhorizon.Command{
		func() eventhorizon.Command { return &account.RegisterNewAccount{} },
		func() eventhorizon.Command { return &account.StartNextMonth{} },
	}

	for _, command := range commands {
		eventhorizon.RegisterCommand(command)
	}
}

func tearDownCommandMediatorTest() {
	eventhorizon.UnregisterCommand(account.RegisterNewAccountCommand)
	eventhorizon.UnregisterCommand(account.StartNextMonthCommand)
}

func TestCommandMediator_RegisterNewAccount(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	transferObject := RegisterNewAccountTransferObject{
		BankName:            string(bankName),
		Name:                name,
		AccountType:         string(accountType),
		StartingBalance:     startingBalance,
		StartingBalanceDate: startingBalanceDate,
		Currency:            string(currency),
		Notes:               &notes,
	}

	t.Run("correctly handles register new account", func(t *testing.T) {
		expectedCommand := &account.RegisterNewAccount{
			AccountId:           *expectedAccountId,
			BankName:            bankName,
			Name:                name,
			AccountType:         accountType,
			StartingBalance:     startingBalance,
			StartingBalanceDate: startingBalanceDate,
			Currency:            currency,
			Notes:               &notes,
		}

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				asserts.Equal(expectedCommand, command)

				return nil
			},
		}
		readModelRepository := &accountreadmodel.ReadModelRepositoryMock{
			GetByNameFn: func(ctx context.Context, name string) (*accountreadmodel.Entity, error) {
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
		requires.Nil(err)

		asserts.Equal(expectedAccountId, accountId)
	})

	failureTestCases := [...]struct {
		testName            string
		commandHandler      *mocks.CommandHandlerMock
		readModelRepository *accountreadmodel.ReadModelRepositoryMock
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
			&accountreadmodel.ReadModelRepositoryMock{
				GetByNameFn: func(ctx context.Context, name string) (*accountreadmodel.Entity, error) {
					return &accountreadmodel.Entity{
						AccountId:           expectedAccountId,
						BankName:            bankName,
						Name:                name,
						AccountType:         accountType,
						StartingBalance:     startingBalance,
						StartingBalanceDate: startingBalanceDate,
						Currency:            currency,
						Notes:               &notes,
						ActiveMonth:         accountreadmodel.EntityActiveMonth{},
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
					asserts.Equal(*expectedAccountId, command.AggregateID())

					return fmt.Errorf("an error")
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByNameFn: func(ctx context.Context, name string) (*accountreadmodel.Entity, error) {
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
			requires.Error(err)
			asserts.Nil(accountId)
		})
	}
}

func TestCommandMediator_StartNextMonth(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	t.Run("successfully starts next month on account", func(t *testing.T) {
		expectedCommand := &account.StartNextMonth{AccountId: *expectedAccountId}

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				asserts.Equal(expectedCommand, command)

				return nil
			},
		}

		readModelRepository := &accountreadmodel.ReadModelRepositoryMock{
			GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
				asserts.Equal(expectedAccountId, accountId)

				return &accountreadmodel.Entity{
					AccountId:           accountId,
					BankName:            bankName,
					Name:                name,
					AccountType:         accountType,
					StartingBalance:     startingBalance,
					StartingBalanceDate: startingBalanceDate,
					Currency:            currency,
					Notes:               &notes,
					ActiveMonth:         accountreadmodel.EntityActiveMonth{},
				}, nil
			},
		}

		commandMediator := NewCommandMediator(commandHandler, readModelRepository, nil)

		err := commandMediator.StartNextMonth(&gin.Context{}, expectedAccountId)
		requires.Nil(err)
	})

	failureTestCases := [...]struct {
		testName            string
		commandHandler      *mocks.CommandHandlerMock
		readModelRepository *accountreadmodel.ReadModelRepositoryMock
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
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					asserts.Equal(expectedAccountId, accountId)

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
					asserts.Equal(*expectedAccountId, command.AggregateID())

					return fmt.Errorf("an error")
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					asserts.Equal(expectedAccountId, accountId)

					return &accountreadmodel.Entity{
						AccountId:           accountId,
						BankName:            bankName,
						Name:                name,
						AccountType:         accountType,
						StartingBalance:     startingBalance,
						StartingBalanceDate: startingBalanceDate,
						Currency:            currency,
						Notes:               &notes,
						ActiveMonth:         accountreadmodel.EntityActiveMonth{},
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

			err := commandMediator.StartNextMonth(&gin.Context{}, expectedAccountId)
			requires.Error(err)
		})
	}
}
