package tagcategory_test

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
	"walletaccountant/tagcategory"
)

var expectedCommand = &tagcategory.AddNewTagToNewCategory{
	TagCategoryId: expectedTagCategoryId,
	Name:          tagCategoryName,
	Notes:         tagCategoryNotes,
	Tag: tagcategory.NewTag{
		TagId: expectedTagId,
		Name:  tagName,
		Notes: tagNotes,
	},
}

func setupCommandMediatorTest() {
	commands := []func() eventhorizon.Command{
		func() eventhorizon.Command { return &tagcategory.AddNewTagToNewCategory{} },
		func() eventhorizon.Command { return &tagcategory.AddNewTagToExistingCategory{} },
	}

	for _, command := range commands {
		eventhorizon.RegisterCommand(command)
	}
}

func tearDownCommandMediatorTest() {
	eventhorizon.UnregisterCommand(tagcategory.AddNewTagToNewCategoryCommand)
	eventhorizon.UnregisterCommand(tagcategory.AddNewTagToExistingCategoryCommand)
}

func TestCommandMediator_AddNewTagToNewCategory(t *testing.T) {
	setupCommandMediatorTest()
	defer tearDownCommandMediatorTest()

	asserts := assert.New(t)
	requires := require.New(t)

	transferObject := tagcategory.AddNewTagToNewCategoryTransferObject{
		CategoryName:  tagCategoryName,
		CategoryNotes: tagCategoryNotes,
		TagName:       tagName,
		TagNotes:      tagNotes,
	}

	t.Run("correctly handles adds new tag to a new tag category", func(t *testing.T) {
		commandHandler := &mocks.CommandHandlerMock{
			HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
				asserts.Equal(expectedCommand, command)

				return nil
			},
		}
		readModelRepository := &tagcategory.ReadModelRepositoryMock{}

		idCreatorCalled := 0
		idCreator := &eventstoredb.IdCreatorMock{
			NewFn: func() uuid.UUID {
				var newId uuid.UUID
				if idCreatorCalled == 0 {
					newId = newTagCategoryId
				} else {
					newId = newTagId
				}

				idCreatorCalled++

				return newId
			},
		}

		commandMediator := tagcategory.NewCommandMediator(commandHandler, readModelRepository, idCreator)

		tagId, tagCategoryId, err := commandMediator.AddNewTagToNewCategory(&gin.Context{}, transferObject)
		requires.Nil(err)

		asserts.Equal(&expectedTagCategoryId, tagCategoryId)
		asserts.Equal(&expectedTagId, tagId)
	})

	failureTestCases := [...]struct {
		testName            string
		commandHandler      *mocks.CommandHandlerMock
		readModelRepository *tagcategory.ReadModelRepositoryMock
		idCreator           *eventstoredb.IdCreatorMock
	}{
		{
			"fails to handle add new tag to new tag category, because category name already exists",
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					asserts.Fail("handle command should not be called on a failure")

					return nil
				},
			},
			&tagcategory.ReadModelRepositoryMock{
				CategoryExistsByNameFn: func(ctx context.Context, name string) (bool, error) {
					return true, nil
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
			"fails to handle add new tag to new tag category, because tag name already exists",
			&mocks.CommandHandlerMock{
				HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
					asserts.Fail("handle command should not be called on a failure")

					return nil
				},
			},
			&tagcategory.ReadModelRepositoryMock{
				ExistsByNameFn: func(ctx context.Context, name string) (bool, error) {
					return true, nil
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
					return fmt.Errorf("an error")
				},
			},
			&tagcategory.ReadModelRepositoryMock{},
			&eventstoredb.IdCreatorMock{
				NewFn: func() uuid.UUID {
					return uuid.New()
				},
			},
		},
	}

	for _, testCase := range failureTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			commandMediator := tagcategory.NewCommandMediator(
				testCase.commandHandler,
				testCase.readModelRepository,
				testCase.idCreator,
			)

			tagId, tagCategoryId, err := commandMediator.AddNewTagToNewCategory(&gin.Context{}, transferObject)
			requires.Error(err)
			asserts.Nil(tagId)
			asserts.Nil(tagCategoryId)
		})
	}
}
