package accountmonthcommand_test

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/accountmonthcommand"
	"walletaccountant/accountmonthreadmodel"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypereadmodel"
	"walletaccountant/tagcategory"
)

func setupCommandMediatorTest() {
	commands := []func() eventhorizon.Command{
		func() eventhorizon.Command { return &accountmonth.RegisterNewAccountMovement{} },
		func() eventhorizon.Command { return &accountmonth.StartAccountMonth{} },
		func() eventhorizon.Command { return &accountmonth.EndAccountMonth{} },
	}

	for _, command := range commands {
		eventhorizon.RegisterCommand(command)
	}
}

func tearDownCommandMediatorTest() {
	eventhorizon.UnregisterCommand(accountmonth.RegisterNewAccountMovementCommand)
	eventhorizon.UnregisterCommand(accountmonth.StartAccountMonthCommand)
	eventhorizon.UnregisterCommand(accountmonth.EndAccountMonthCommand)
}

func TestCommandMediator_RegisterNewAccountMovement(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	transferObject := accountmonthcommand.RegisterNewAccountMovementTransferObject{
		AccountId:       accountId1.String(),
		MovementTypeId:  stringPtr(movementTypeId1.String()),
		Amount:          101000,
		Date:            date,
		Action:          "credit",
		SourceAccountId: nil,
		Description:     "My movement description",
		Notes:           nil,
		TagIds:          []string{tagId1.String()},
	}

	expectedCommand := &accountmonth.RegisterNewAccountMovement{
		AccountMonthId:    *accountMonthId,
		AccountMovementId: *accountMovementId,
		MovementTypeId:    movementTypeId1,
		Action:            common.Credit,
		Amount:            101000,
		Date:              date,
		SourceAccountId:   nil,
		Description:       "My movement description",
		Notes:             nil,
		TagIds:            []*tagcategory.TagId{&tagId1},
	}

	var accountByIdCalled int
	var movementTypeByIdCalled int
	successTestCases := [...]struct {
		testName                        string
		accountReadModelRepository      *accountreadmodel.ReadModelRepositoryMock
		movementTypeReadModelRepository *movementtypereadmodel.ReadModelRepositoryMock
	}{
		{
			testName: "correctly handles register new account movement",
			accountReadModelRepository: &accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					requires.GreaterOrEqual(1, accountByIdCalled)

					asserts.Equal(transferObject.AccountId, accountId.String())

					return &accountEntity, nil
				},
			},
			movementTypeReadModelRepository: &movementtypereadmodel.ReadModelRepositoryMock{
				GetByMovementTypeIdFn: func(
					ctx context.Context,
					movementTypeId *movementtype.Id,
				) (*movementtypereadmodel.Entity, error) {
					movementTypeByIdCalled++

					requires.GreaterOrEqual(1, movementTypeByIdCalled)

					asserts.Equal(expectedCommand.MovementTypeId, movementTypeId)

					return &movementTypeEntity, nil
				},
			},
		},
	}
	for _, testCase := range successTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			accountByIdCalled = 0
			movementTypeByIdCalled = 0

			commandHandler := &mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					asserts.Equal(expectedCommand, command)

					return nil
				},
			}

			idCreator := &eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return *accountMovementId
				},
			}

			commandMediator := accountmonthcommand.NewCommandMediator(
				commandHandler,
				&accountmonthreadmodel.ReadModelRepositoryMock{},
				testCase.accountReadModelRepository,
				testCase.movementTypeReadModelRepository,
				idCreator,
			)

			err := commandMediator.RegisterNewAccountMovement(&gin.Context{}, transferObject)
			requires.Nil(err)

			asserts.Equal(
				1,
				accountByIdCalled,
				"account by id should be called 1 time, called %d",
				accountByIdCalled,
			)
			asserts.Equal(
				1,
				movementTypeByIdCalled,
				"movement type by id should be called 1 time, called %d",
				movementTypeByIdCalled,
			)
		})
	}

	failureTestCases := [...]struct {
		testName                        string
		expectedAccountByIdCalled       int
		expectedMovementTypeByIdCalled  int
		transferObject                  accountmonthcommand.RegisterNewAccountMovementTransferObject
		expectedErrorReason             definitions.ErrorReason
		readModelRepository             *accountmonthreadmodel.ReadModelRepositoryMock
		accountReadModelRepository      *accountreadmodel.ReadModelRepositoryMock
		movementTypeReadModelRepository *movementtypereadmodel.ReadModelRepositoryMock
	}{
		{
			"fails to handle register new account movement, because account does not exist",
			1,
			0,
			transferObject,
			accountmonth.NonExistentAccount,
			&accountmonthreadmodel.ReadModelRepositoryMock{},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					return nil, nil
				},
			},
			&movementtypereadmodel.ReadModelRepositoryMock{
				GetByMovementTypeIdFn: func(
					ctx context.Context,
					movementTypeId *movementtype.Id,
				) (*movementtypereadmodel.Entity, error) {
					movementTypeByIdCalled++

					return nil, nil
				},
			},
		},
		{
			"fails to handle register new account movement, because of active month mismatch",
			1,
			0,
			transferObject,
			accountmonth.MismatchedActiveMonth,
			&accountmonthreadmodel.ReadModelRepositoryMock{},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					return &accountEntity2, nil
				},
			},
			&movementtypereadmodel.ReadModelRepositoryMock{
				GetByMovementTypeIdFn: func(
					ctx context.Context,
					movementTypeId *movementtype.Id,
				) (*movementtypereadmodel.Entity, error) {
					movementTypeByIdCalled++

					return nil, nil
				},
			},
		},
		{
			"fails to handle register new account movement, because movement type does not exist",
			1,
			1,
			transferObject,
			accountmonth.NonExistentMovementType,
			&accountmonthreadmodel.ReadModelRepositoryMock{},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					return &accountEntity, nil
				},
			},
			&movementtypereadmodel.ReadModelRepositoryMock{
				GetByMovementTypeIdFn: func(
					ctx context.Context,
					movementTypeId *movementtype.Id,
				) (*movementtypereadmodel.Entity, error) {
					movementTypeByIdCalled++

					return nil, nil
				},
			},
		},
		{
			"fails to handle register new account movement, because account and movement type mismatch",
			1,
			1,
			transferObject,
			accountmonth.MismatchedAccountId,
			&accountmonthreadmodel.ReadModelRepositoryMock{},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					return &accountEntity, nil
				},
			},
			&movementtypereadmodel.ReadModelRepositoryMock{
				GetByMovementTypeIdFn: func(
					ctx context.Context,
					movementTypeId *movementtype.Id,
				) (*movementtypereadmodel.Entity, error) {
					movementTypeByIdCalled++

					return &movementTypeEntity2, nil
				},
			},
		},
	}

	for _, testCase := range failureTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			handleCommandCalled := 0
			accountByIdCalled = 0
			movementTypeByIdCalled = 0

			idCreator := &eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return *accountMovementId
				},
			}

			commandMediator := accountmonthcommand.NewCommandMediator(
				&mocks.CommandHandlerMock{
					HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
						handleCommandCalled++

						return nil
					},
				},
				&accountmonthreadmodel.ReadModelRepositoryMock{},
				testCase.accountReadModelRepository,
				testCase.movementTypeReadModelRepository,
				idCreator,
			)

			err := commandMediator.RegisterNewAccountMovement(&gin.Context{}, testCase.transferObject)
			requires.Error(err)
			asserts.IsType(&definitions.WalletAccountantError{}, err)
			asserts.Equal(testCase.expectedErrorReason, err.Reason)
			asserts.Equalf(
				0,
				handleCommandCalled,
				"command handler should called 0 times, called %d",
				handleCommandCalled,
			)
			asserts.Equal(
				testCase.expectedAccountByIdCalled,
				accountByIdCalled,
				"account by id should called %d, called %d",
				testCase.expectedAccountByIdCalled,
				accountByIdCalled,
			)
			asserts.Equal(
				testCase.expectedMovementTypeByIdCalled,
				movementTypeByIdCalled,
				"tag exists by id should called %d, called %d",
				testCase.expectedMovementTypeByIdCalled,
				movementTypeByIdCalled,
			)
		})
	}
}

func TestCommandMediator_EndAccountMonth(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	endBalance1 := int64(103056)
	endBalance2 := int64(103000)

	transferObject := accountmonthcommand.EndAccountMonthTransferObject{
		AccountId:  accountId1.String(),
		EndBalance: &endBalance1,
		Month:      month,
		Year:       year,
	}
	transferObjectDifferentBalance := accountmonthcommand.EndAccountMonthTransferObject{
		AccountId:  accountId1.String(),
		EndBalance: &endBalance2,
		Month:      month,
		Year:       year,
	}

	expectedCommand := &accountmonth.EndAccountMonth{
		AccountMonthId: *accountMonthId,
		AccountId:      *accountId1,
		EndBalance:     103056,
		Month:          month,
		Year:           year,
	}

	var accountByIdCalled int
	var accountMonthByActiveMonthCalled int
	t.Run("correctly handles end account month", func(t *testing.T) {
		accountByIdCalled = 0
		accountMonthByActiveMonthCalled = 0

		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				asserts.Equal(expectedCommand, command)

				return nil
			},
		}

		idCreator := &eventstoredb.IdCreatorMock{
			NewFn: func() uuid.UUID {
				return *accountMovementId
			},
		}

		commandMediator := accountmonthcommand.NewCommandMediator(
			commandHandler,
			&accountmonthreadmodel.ReadModelRepositoryMock{
				GetByAccountActiveMonthFn: func(
					ctx context.Context,
					account *accountreadmodel.Entity,
				) (*accountmonthreadmodel.Entity, error) {
					accountMonthByActiveMonthCalled++

					return &accountMonthEntity, nil
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					requires.GreaterOrEqual(1, accountByIdCalled)

					asserts.Equal(transferObject.AccountId, accountId.String())

					return &accountEntity, nil
				},
			},
			&movementtypereadmodel.ReadModelRepositoryMock{},
			idCreator,
		)

		err := commandMediator.EndAccountMonth(&gin.Context{}, transferObject)
		requires.Nil(err)

		asserts.Equal(
			1,
			accountByIdCalled,
			"account by id should be called 1 time, called %d",
			accountByIdCalled,
		)

		asserts.Equal(
			1,
			accountMonthByActiveMonthCalled,
			"account by id should be called 1 time, called %d",
			accountMonthByActiveMonthCalled,
		)
	})

	failureTestCases := [...]struct {
		testName                                string
		expectedAccountByIdCalled               int
		expectedAccountMonthByActiveMonthCalled int
		transferObject                          accountmonthcommand.EndAccountMonthTransferObject
		expectedErrorReason                     definitions.ErrorReason
		readModelRepository                     *accountmonthreadmodel.ReadModelRepositoryMock
		accountReadModelRepository              *accountreadmodel.ReadModelRepositoryMock
	}{
		{
			"fails to handle end account month, because account does not exist",
			1,
			0,
			transferObject,
			accountmonth.NonExistentAccount,
			&accountmonthreadmodel.ReadModelRepositoryMock{
				GetByAccountActiveMonthFn: func(
					ctx context.Context,
					account *accountreadmodel.Entity,
				) (*accountmonthreadmodel.Entity, error) {
					accountMonthByActiveMonthCalled++

					return nil, nil
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					asserts.Equal(accountId1, accountId)

					return nil, nil
				},
			},
		},
		{
			"fails to handle end account month, because of active month mismatch",
			1,
			0,
			transferObject,
			accountmonth.MismatchedActiveMonth,
			&accountmonthreadmodel.ReadModelRepositoryMock{
				GetByAccountActiveMonthFn: func(
					ctx context.Context,
					account *accountreadmodel.Entity,
				) (*accountmonthreadmodel.Entity, error) {
					accountMonthByActiveMonthCalled++

					return nil, nil
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					asserts.Equal(accountId1, accountId)

					return &accountEntity2, nil
				},
			},
		},
		{
			"fails to handle end account month, because account month does not exist",
			1,
			1,
			transferObject,
			accountmonth.NonExistentAccountMonth,
			&accountmonthreadmodel.ReadModelRepositoryMock{
				GetByAccountActiveMonthFn: func(
					ctx context.Context,
					account *accountreadmodel.Entity,
				) (*accountmonthreadmodel.Entity, error) {
					accountMonthByActiveMonthCalled++

					asserts.Equal(accountId1, account.AccountId)

					return nil, nil
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					asserts.Equal(accountId1, accountId)

					return &accountEntity, nil
				},
			},
		},
		{
			"fails to handle end account month, because account month already ended",
			1,
			1,
			transferObject,
			accountmonth.AlreadyEnded,
			&accountmonthreadmodel.ReadModelRepositoryMock{
				GetByAccountActiveMonthFn: func(
					ctx context.Context,
					account *accountreadmodel.Entity,
				) (*accountmonthreadmodel.Entity, error) {
					accountMonthByActiveMonthCalled++

					asserts.Equal(accountId1, account.AccountId)

					return &accountMonthEntityEnded, nil
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					asserts.Equal(accountId1, accountId)

					return &accountEntity, nil
				},
			},
		},
		{
			"fails to handle end account month, because account balances are different",
			1,
			1,
			transferObjectDifferentBalance,
			accountmonth.MismatchedEndBalance,
			&accountmonthreadmodel.ReadModelRepositoryMock{
				GetByAccountActiveMonthFn: func(
					ctx context.Context,
					account *accountreadmodel.Entity,
				) (*accountmonthreadmodel.Entity, error) {
					accountMonthByActiveMonthCalled++

					asserts.Equal(accountId1, account.AccountId)

					return &accountMonthEntity, nil
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					asserts.Equal(accountId1, accountId)

					return &accountEntity, nil
				},
			},
		},
	}

	for _, testCase := range failureTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			handleCommandCalled := 0
			accountByIdCalled = 0
			accountMonthByActiveMonthCalled = 0

			idCreator := &eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return *accountMovementId
				},
			}

			commandMediator := accountmonthcommand.NewCommandMediator(
				&mocks.CommandHandlerMock{
					HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
						handleCommandCalled++

						return nil
					},
				},
				testCase.readModelRepository,
				testCase.accountReadModelRepository,
				&movementtypereadmodel.ReadModelRepositoryMock{},
				idCreator,
			)

			err := commandMediator.EndAccountMonth(&gin.Context{}, testCase.transferObject)
			requires.Error(err)
			asserts.IsType(&definitions.WalletAccountantError{}, err)
			asserts.Equal(testCase.expectedErrorReason, err.Reason)
			asserts.Equalf(
				0,
				handleCommandCalled,
				"command handler should not be called, called %d",
				handleCommandCalled,
			)
			asserts.Equal(
				testCase.expectedAccountByIdCalled,
				accountByIdCalled,
				"account by id should called %d, called %d",
				testCase.expectedAccountByIdCalled,
				accountByIdCalled,
			)
			asserts.Equal(
				testCase.expectedAccountMonthByActiveMonthCalled,
				accountMonthByActiveMonthCalled,
				"account month by active month should called %d, called %d",
				testCase.expectedAccountMonthByActiveMonthCalled,
				accountMonthByActiveMonthCalled,
			)
		})
	}
}
