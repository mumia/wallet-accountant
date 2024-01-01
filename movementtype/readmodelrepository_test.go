package movementtype_test

import (
	"context"
	googleUUID "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"testing"
	"walletaccountant/common"
	"walletaccountant/mongodb"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

func TestReadModelRepository_Create(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test successfully register new movement type", func(mt *mtest.T) {
		readModelRepository := movementtype.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := readModelRepository.Create(context.Background(), &movementTypeEntity1)
		requires.NoError(err)

		assertCreate1(assertEventsForInsert(mt, asserts, requires), asserts)
	})

	mt.Run("test successfully register new movement type with source account", func(mt *mtest.T) {
		readModelRepository := movementtype.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := readModelRepository.Create(context.Background(), &movementTypeEntityWithSourceAccount)
		requires.NoError(err)

		assertCreateWithSourceAccount(assertEventsForInsert(mt, asserts, requires), asserts)
	})

	mt.Run("test failure add tag to new category", func(mt *mtest.T) {
		readModelRepository := movementtype.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(
				mtest.WriteError{
					Index:   0,
					Code:    0,
					Message: "an error",
				},
			),
		)

		err := readModelRepository.Create(context.Background(), &movementTypeEntity1)
		requires.Error(err)

		assertCreate1(assertEventsForInsert(mt, asserts, requires), asserts)
	})
}

func TestReadModelRepository_GetAll(t *testing.T) {
	t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses, see account test")
}

func TestReadModelRepository_GetByMovementTypeId(t *testing.T) {
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

//func assertEventsForUpdate(mt *mtest.T, asserts *assert.Assertions, requires *require.Assertions) bson.Raw {
//	events := mt.GetAllSucceededEvents()
//	requires.Len(events, 1)
//	event := events[0]
//	asserts.Equal("update", event.CommandName)
//
//	startedEvents := mt.GetAllStartedEvents()
//	requires.Len(startedEvents, 1)
//	startedEvent := startedEvents[0]
//	elements, err := startedEvent.Command.Elements()
//	requires.NoError(err)
//
//	var update bson.Raw
//	for _, element := range elements {
//		switch element.Key() {
//		case "update":
//			asserts.Equal(tagcategory.AggregateType.String(), element.Value().StringValue())
//		case "$db":
//			asserts.Equal(mongodb.DatabaseName, element.Value().StringValue())
//		case "updates":
//			filter := element.Value().Array().Lookup("0").Document().Lookup("q").Document()
//			assertBinaryId(filter.Lookup("_id"), expectedTagCategoryId.String(), asserts)
//
//			update = element.Value().Array().Lookup("0").Document().Lookup("u").Document()
//		}
//	}
//
//	return update
//}

func assertCreate1(command bson.Raw, asserts *assert.Assertions) {
	assertBinaryId(command.Lookup("_id"), movementTypeId1.String(), asserts)
	asserts.Equal(string(common.Credit), command.Lookup("action").StringValue())
	assertBinaryId(command.Lookup("account_id"), accountId1.String(), asserts)
	asserts.True(command.Lookup("source_account_id").IsZero())
	asserts.Equal(description, command.Lookup("description").StringValue())
	asserts.Equal(notes, command.Lookup("notes").StringValue())
	assertTag1(command.Lookup("tags"), 0, asserts)
}

func assertCreateWithSourceAccount(command bson.Raw, asserts *assert.Assertions) {
	assertBinaryId(command.Lookup("_id"), movementTypeId2.String(), asserts)
	asserts.Equal(string(common.Debit), command.Lookup("action").StringValue())
	assertBinaryId(command.Lookup("account_id"), accountId2.String(), asserts)
	assertBinaryId(command.Lookup("source_account_id"), sourceAccountId.String(), asserts)
	asserts.Equal(description2, command.Lookup("description").StringValue())
	asserts.Equal(notes2, command.Lookup("notes").StringValue())
	assertTag1(command.Lookup("tags"), 1, asserts)
	assertTag2(command.Lookup("tags"), 0, asserts)
}

func assertBinaryId(idValue bson.RawValue, expectedId string, asserts *assert.Assertions) {
	_, data := idValue.Binary()

	actualUuid, err := googleUUID.FromBytes(data)
	asserts.NoError(err)

	asserts.Equal(expectedId, actualUuid.String())
}

func assertTag1(tags bson.RawValue, index uint, asserts *assert.Assertions) {
	assertBinaryId(tags.Array().Index(index).Value(), tagId1.String(), asserts)
}

func assertTag2(tags bson.RawValue, index uint, asserts *assert.Assertions) {
	assertBinaryId(tags.Array().Index(index).Value(), tagId2.String(), asserts)
}
