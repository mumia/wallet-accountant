package movementtypecommand_test

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"testing"
	"walletaccountant/account"
	"walletaccountant/accountreadmodel"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypecommand"
	"walletaccountant/tagcategory"
)

func setupCommandMediatorTest() {
	commands := []func() eventhorizon.Command{
		func() eventhorizon.Command { return &movementtype.RegisterNewMovementType{} },
	}

	for _, command := range commands {
		eventhorizon.RegisterCommand(command)
	}
}

func tearDownCommandMediatorTest() {
	eventhorizon.UnregisterCommand(movementtype.RegisterNewMovementTypeCommand)
}

func TestCommandMediator_RegisterNewMovementType(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	transferObject := movementtypecommand.RegisterNewMovementTypeTransferObject{
		Action:          string(movementType),
		AccountId:       accountId1.String(),
		SourceAccountId: nil,
		Description:     description,
		Notes:           &notes,
		TagIds:          []string{tagId1.String()},
	}
	accountId1String := accountId1.String()
	transferObjectWithSameAccounts := movementtypecommand.RegisterNewMovementTypeTransferObject{
		Action:          string(movementType),
		AccountId:       accountId1.String(),
		SourceAccountId: &accountId1String,
		Description:     description,
		Notes:           &notes,
		TagIds:          []string{tagId1.String()},
	}

	expectedCommand := &movementtype.RegisterNewMovementType{
		MovementTypeId:  movementTypeId1,
		Action:          movementType,
		AccountId:       accountId1,
		SourceAccountId: nil,
		Description:     description,
		Notes:           &notes,
		TagIds:          []*tagcategory.TagId{&tagId1},
	}

	var accountByIdCalled int
	var tagExistsByIdCalled int
	accountEntity := accountreadmodel.Entity{}
	successTestCases := [...]struct {
		testName                       string
		movementTypeId                 *movementtype.Id
		accountReadModelRepository     *accountreadmodel.ReadModelRepositoryMock
		tagCategoryReadModelRepository *tagcategory.ReadModelRepositoryMock
	}{
		{
			testName:       "correctly handles register new movement type",
			movementTypeId: &movementTypeId1,
			accountReadModelRepository: &accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					requires.GreaterOrEqual(1, accountByIdCalled)

					asserts.Equal(expectedCommand.AccountId.String(), accountId.String())

					return &accountEntity, nil
				},
			},
			tagCategoryReadModelRepository: &tagcategory.ReadModelRepositoryMock{
				ExistsByIdFn: func(ctx context.Context, tagId *tagcategory.TagId) (bool, error) {
					tagExistsByIdCalled++

					requires.GreaterOrEqual(len(expectedCommand.TagIds), tagExistsByIdCalled)

					asserts.Equal(expectedCommand.TagIds[tagExistsByIdCalled-1].String(), tagId.String())

					return true, nil
				},
			},
		},
		{
			testName:       "correctly handles register new movement type, with source account",
			movementTypeId: &movementTypeId2,
			accountReadModelRepository: &accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					requires.GreaterOrEqual(2, accountByIdCalled)

					if accountByIdCalled == 1 {
						asserts.Equal(expectedCommand.AccountId.String(), accountId.String())
					} else {
						asserts.Equal(expectedCommand.SourceAccountId.String(), accountId.String())
					}

					return &accountEntity, nil
				},
			},
			tagCategoryReadModelRepository: &tagcategory.ReadModelRepositoryMock{
				ExistsByIdFn: func(ctx context.Context, tagId *tagcategory.TagId) (bool, error) {
					tagExistsByIdCalled++

					requires.GreaterOrEqual(len(expectedCommand.TagIds), tagExistsByIdCalled)

					asserts.Equal(expectedCommand.TagIds[tagExistsByIdCalled-1].String(), tagId.String())

					return true, nil
				},
			},
		},
	}
	for _, testCase := range successTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			accountByIdCalled = 0
			tagExistsByIdCalled = 0

			commandHandler := &mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					asserts.Equal(expectedCommand, command)

					return nil
				},
			}

			idCreator := &eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return movementEventUUID1
				},
			}

			commandMediator := movementtypecommand.NewCommandMediator(
				commandHandler,
				testCase.accountReadModelRepository,
				testCase.tagCategoryReadModelRepository,
				idCreator,
			)

			actualMovementTypeId, err := commandMediator.RegisterNewMovementType(&gin.Context{}, transferObject)
			requires.Nil(err)

			asserts.Equal(movementEventUUID1.String(), actualMovementTypeId.String())
			asserts.Equal(
				1,
				accountByIdCalled,
				"account by id should called 1, called %d",
				accountByIdCalled,
			)
			asserts.Equal(len(expectedCommand.TagIds), tagExistsByIdCalled)
		})
	}

	var handleCommandCalled int
	failureTestCases := [...]struct {
		testName                       string
		expectedHandleCommandCalled    int
		expectedAccountByIdCalled      int
		expectedTagExistsByIdCalled    int
		transferObject                 movementtypecommand.RegisterNewMovementTypeTransferObject
		expectedErrorReason            definitions.ErrorReason
		commandHandler                 *mocks.CommandHandlerMock
		accountReadModelRepository     *accountreadmodel.ReadModelRepositoryMock
		tagCategoryReadModelRepository *tagcategory.ReadModelRepositoryMock
		idCreator                      *eventstoredb.IdCreatorMock
	}{
		{
			"fails to handle register new action, because of account and source account are the same",
			0,
			0,
			0,
			transferObjectWithSameAccounts,
			movementtype.SameAccountAndSourceAccount,
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					handleCommandCalled++

					return nil
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					return nil, nil
				},
			},
			&tagcategory.ReadModelRepositoryMock{
				ExistsByIdFn: func(ctx context.Context, tagId *tagcategory.TagId) (bool, error) {
					tagExistsByIdCalled++

					return false, nil
				},
			},
			&eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return movementEventUUID1
				},
			},
		},
		{
			"fails to handle register new action, because of account not found",
			0,
			1,
			0,
			transferObject,
			movementtype.NonExistentMovementTypeAccount,
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					handleCommandCalled++

					return nil
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					return nil, mongo.ErrNoDocuments
				},
			},
			&tagcategory.ReadModelRepositoryMock{
				ExistsByIdFn: func(ctx context.Context, tagId *tagcategory.TagId) (bool, error) {
					tagExistsByIdCalled++

					return false, nil
				},
			},
			&eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return movementEventUUID1
				},
			},
		},
		{
			"fails to handle register new action, because of tags not found",
			0,
			1,
			1,
			transferObject,
			movementtype.NonExistentMovementTypeTag,
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					handleCommandCalled++

					return nil
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					return &accountreadmodel.Entity{}, nil
				},
			},
			&tagcategory.ReadModelRepositoryMock{
				ExistsByIdFn: func(ctx context.Context, tagId *tagcategory.TagId) (bool, error) {
					tagExistsByIdCalled++

					return false, nil
				},
			},
			&eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return movementEventUUID1
				},
			},
		},
		{
			"fails to handle register new action, because of err on command handler",
			1,
			1,
			1,
			transferObject,
			"an error",
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					handleCommandCalled++

					asserts.Equal(movementTypeId1.String(), command.AggregateID().String())

					return fmt.Errorf("an error")
				},
			},
			&accountreadmodel.ReadModelRepositoryMock{
				GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
					accountByIdCalled++

					return &accountreadmodel.Entity{}, nil
				},
			},
			&tagcategory.ReadModelRepositoryMock{
				ExistsByIdFn: func(ctx context.Context, tagId *tagcategory.TagId) (bool, error) {
					tagExistsByIdCalled++

					return true, nil
				},
			},
			&eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return movementEventUUID1
				},
			},
		},
	}

	for _, testCase := range failureTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			handleCommandCalled = 0
			accountByIdCalled = 0
			tagExistsByIdCalled = 0

			commandMediator := movementtypecommand.NewCommandMediator(
				testCase.commandHandler,
				testCase.accountReadModelRepository,
				testCase.tagCategoryReadModelRepository,
				testCase.idCreator,
			)

			movementTypeId, err := commandMediator.RegisterNewMovementType(&gin.Context{}, testCase.transferObject)
			requires.Error(err)
			asserts.IsType(&definitions.WalletAccountantError{}, err)
			asserts.Equal(testCase.expectedErrorReason, err.Reason)
			asserts.Nil(movementTypeId)
			asserts.Equalf(
				testCase.expectedHandleCommandCalled,
				handleCommandCalled,
				"command handler should called %d, called %d",
				testCase.expectedHandleCommandCalled,
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
				testCase.expectedTagExistsByIdCalled,
				tagExistsByIdCalled,
				"tag exists by id should called %d, called %d",
				testCase.expectedTagExistsByIdCalled,
				tagExistsByIdCalled,
			)
		})
	}
}
