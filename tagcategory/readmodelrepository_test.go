package tagcategory_test

import (
	"context"
	googleUUID "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"testing"
	"walletaccountant/mongodb"
	"walletaccountant/tagcategory"
)

//var tagCategoryIdBson1, _ = bson.Marshal(expectedTagCategoryId)
//var tagIdBson1, _ = bson.Marshal(expectedTagId)

//var tagCategoryBson1 = bson.D{
//	{"_id", tagCategoryIdBson1},
//	{"name", tagCategoryName},
//	{"notes", tagCategoryNotes},
//	{"tags",
//		bson.A{
//			bson.D{
//				{"_id", tagIdBson1},
//				{"name", tagName},
//				{"notes", tagNotes},
//			},
//		},
//	},
//}

//var expectedAccountId2 = account.TagId(uuid.New())
//var expectedAccountEntity2 = account.Entity{
//	AccountId:           &expectedAccountId1,
//	BankName:            "another bank name",
//	Name:                "annother account name",
//	AccountType:         account.Savings,
//	StartingBalance:     6069,
//	StartingBalanceDate: time.Now().Add(1 * time.Minute),
//	Currency:            account.USD,
//	Notes:               "another set of notes",
//	ActiveMonth: account.EntityActiveMonth{
//		Month: time.April,
//		Year:  2022,
//	},
//}
//
//var accountIdBson2, _ = bson.Marshal(expectedAccountId2)
//var accountBson2 = bson.D{
//	{"_id", accountIdBson2},
//	{"bank_name", expectedAccountEntity2.BankName},
//	{"name", expectedAccountEntity2.Name},
//	{"account_type", expectedAccountEntity2.AccountType},
//	{"starting_balance", expectedAccountEntity2.StartingBalance},
//	{"starting_balance_date", expectedAccountEntity2.StartingBalanceDate},
//	{"currency", expectedAccountEntity2.Currency},
//	{"notes", expectedAccountEntity2.Notes},
//	{
//		"active_month",
//		bson.D{
//			{"month", expectedAccountEntity2.ActiveMonth.Month},
//			{"year", expectedAccountEntity2.ActiveMonth.Year},
//		},
//	},
//}

func TestReadModelRepository_AddNewTagAndCategory(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test successful add tag to new category", func(mt *mtest.T) {
		readModelRepository := tagcategory.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := readModelRepository.AddNewTagAndCategory(context.Background(), &tagCategory1)
		requires.NoError(err)

		assertAddNewTagAndCategory(assertEventsForInsert(mt, asserts, requires), asserts)
	})

	mt.Run("test failure add tag to new category", func(mt *mtest.T) {
		readModelRepository := tagcategory.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(
				mtest.WriteError{
					Index:   0,
					Code:    0,
					Message: "an error",
				},
			),
		)

		err := readModelRepository.AddNewTagAndCategory(context.Background(), &tagCategory1)
		requires.Error(err)

		assertAddNewTagAndCategory(assertEventsForInsert(mt, asserts, requires), asserts)
	})
}

func TestReadModelRepository_AddNewTagToCategory(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test successful add tag to existing category", func(mt *mtest.T) {
		readModelRepository := tagcategory.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := readModelRepository.AddNewTagAndCategory(context.Background(), &tagCategory1)
		requires.NoError(err)

		assertAddNewTagAndCategory(assertEventsForInsert(mt, asserts, requires), asserts)
	})

	mt.Run("test failure add tag to existing category", func(mt *mtest.T) {
		readModelRepository := tagcategory.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(
				mtest.WriteError{
					Index:   0,
					Code:    0,
					Message: "an error",
				},
			),
		)

		err := readModelRepository.AddNewTagToCategory(context.Background(), &expectedTagCategoryId, &tag1)
		requires.Error(err)

		assertAddNewTagToCategory(assertEventsForUpdate(mt, asserts, requires), asserts)
	})
}

func TestReadModelRepository_GetAll(t *testing.T) {
	t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses, see account test")
}

func TestReadModelRepository_ExistsByName(t *testing.T) {
	t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses, see account test")
}

func TestReadModelRepository_CategoryExistsById(t *testing.T) {
	t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses, see account test")
}

func TestReadModelRepository_CategoryExistsByName(t *testing.T) {
	t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses, see account test")
}

func assertEventsForInsert(mt *mtest.T, asserts *assert.Assertions, requires *require.Assertions) bson.Raw {
	events := mt.GetAllSucceededEvents()
	requires.Len(events, 1)
	event := events[0]
	asserts.Equal("insert", event.CommandName)

	startedEvents := mt.GetAllStartedEvents()
	requires.Len(startedEvents, 1)
	startedEvent := startedEvents[0]
	elements, err := startedEvent.Command.Elements()
	requires.NoError(err)

	var document bson.Raw
	for _, element := range elements {
		switch element.Key() {
		case "update":
			asserts.Equal(tagcategory.AggregateType.String(), element.Value().StringValue())
		case "$db":
			asserts.Equal(mongodb.DatabaseName, element.Value().StringValue())
		case "documents":
			document = element.Value().Array().Index(0).Value().Document()
		}
	}

	return document
}

func assertEventsForUpdate(mt *mtest.T, asserts *assert.Assertions, requires *require.Assertions) bson.Raw {
	events := mt.GetAllSucceededEvents()
	requires.Len(events, 1)
	event := events[0]
	asserts.Equal("update", event.CommandName)

	startedEvents := mt.GetAllStartedEvents()
	requires.Len(startedEvents, 1)
	startedEvent := startedEvents[0]
	elements, err := startedEvent.Command.Elements()
	requires.NoError(err)

	var update bson.Raw
	for _, element := range elements {
		switch element.Key() {
		case "update":
			asserts.Equal(tagcategory.AggregateType.String(), element.Value().StringValue())
		case "$db":
			asserts.Equal(mongodb.DatabaseName, element.Value().StringValue())
		case "updates":
			filter := element.Value().Array().Lookup("0").Document().Lookup("q").Document()
			assertBinaryId(filter.Lookup("_id"), expectedTagCategoryId.String(), asserts)

			update = element.Value().Array().Lookup("0").Document().Lookup("u").Document()
		}
	}

	return update
}

func assertBinaryId(idValue bson.RawValue, expectedId string, asserts *assert.Assertions) {
	_, data := idValue.Binary()

	actualUuid, err := googleUUID.FromBytes(data)
	asserts.NoError(err)

	asserts.Equal(expectedId, actualUuid.String())
}

func assertAddNewTagAndCategory(command bson.Raw, asserts *assert.Assertions) {
	assertBinaryId(command.Lookup("_id"), expectedTagCategoryId.String(), asserts)

	asserts.Equal(tagCategoryName, command.Lookup("name").StringValue())
	asserts.Equal(tagCategoryNotes, command.Lookup("notes").StringValue())
	assertTags(command.Lookup("tags"), asserts)
}

func assertAddNewTagToCategory(command bson.Raw, asserts *assert.Assertions) {
	command = command.Lookup("$push").Document().Lookup("tags").Document()

	assertBinaryId(command.Lookup("_id"), expectedTagId.String(), asserts)

	asserts.Equal(tagName, command.Lookup("name").StringValue())
	asserts.Equal(tagNotes, command.Lookup("notes").StringValue())
}

func assertTags(tags bson.RawValue, asserts *assert.Assertions) {
	tag := tags.Array().Index(0).Value().Document()

	assertBinaryId(tag.Lookup("_id"), expectedTagId.String(), asserts)

	asserts.Equal(tagName, tag.Lookup("name").StringValue())
	asserts.Equal(tagNotes, tag.Lookup("notes").StringValue())
}
