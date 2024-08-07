package ledgerreadmodel_test

import (
	"context"
	googleUUID "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"testing"
	"walletaccountant/common"
	"walletaccountant/ledger"
	"walletaccountant/ledgerreadmodel"
	"walletaccountant/mongodb"
)

func setupBalance() int64 {
	return 107060
}

func TestReadModelRepository_StartMonth(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test successful start account month", func(mt *mtest.T) {
		readModelRepository := ledgerreadmodel.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := readModelRepository.StartMonth(
			context.Background(),
			accountMonthId,
			accountId1,
			setupBalance(),
			month,
			year,
		)
		requires.NoError(err)

		assertStartMonth(assertEventsForInsert(mt, asserts, requires), asserts)
	})

	mt.Run("test failure to start account month", func(mt *mtest.T) {
		readModelRepository := ledgerreadmodel.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(
				mtest.WriteError{
					Index:   0,
					Code:    0,
					Message: "an error",
				},
			),
		)

		err := readModelRepository.StartMonth(
			context.Background(),
			accountMonthId,
			accountId1,
			setupBalance(),
			month,
			year,
		)
		requires.Error(err)

		assertStartMonth(assertEventsForInsert(mt, asserts, requires), asserts)
	})
}

func TestReadModelRepository_EndMonth(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test successful end account month", func(mt *mtest.T) {
		readModelRepository := ledgerreadmodel.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := readModelRepository.EndMonth(context.Background(), accountMonthId)
		requires.NoError(err)

		assertEndMonth(assertEventsForUpdate(mt, asserts, requires), asserts)
	})

	mt.Run("test failure to start account month", func(mt *mtest.T) {
		readModelRepository := ledgerreadmodel.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(
				mtest.WriteError{
					Index:   0,
					Code:    0,
					Message: "an error",
				},
			),
		)

		err := readModelRepository.EndMonth(context.Background(), accountMonthId)
		requires.Error(err)

		assertEndMonth(assertEventsForUpdate(mt, asserts, requires), asserts)
	})
}

func TestReadModelRepository_RegisterAccountMovement(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("test successful register account movement, debit", func(mt *mtest.T) {
		readModelRepository := ledgerreadmodel.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		eventData := ledger.NewAccountMovementRegisteredData{
			AccountMonthId:  accountMonthId,
			MovementTypeId:  movementTypeId1,
			Action:          common.Debit,
			Amount:          setupBalance(),
			Date:            date,
			SourceAccountId: nil,
			Description:     "",
			Notes:           nil,
			TagIds:          nil,
		}

		err := readModelRepository.RegisterAccountMovement(context.Background(), accountMonthId, &eventData)
		requires.NoError(err)

		assertRegisterAccountMovement(common.Debit, assertEventsForUpdate(mt, asserts, requires), asserts)
	})

	mt.Run("test successful register account movement, credit", func(mt *mtest.T) {
		readModelRepository := ledgerreadmodel.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		eventData := ledger.NewAccountMovementRegisteredData{
			AccountMonthId:  accountMonthId,
			MovementTypeId:  movementTypeId1,
			Action:          common.Credit,
			Amount:          setupBalance(),
			Date:            date,
			SourceAccountId: nil,
			Description:     "",
			Notes:           nil,
			TagIds:          nil,
		}

		err := readModelRepository.RegisterAccountMovement(context.Background(), accountMonthId, &eventData)
		requires.NoError(err)

		assertRegisterAccountMovement(common.Credit, assertEventsForUpdate(mt, asserts, requires), asserts)
	})

	mt.Run("test failure to start account month", func(mt *mtest.T) {
		readModelRepository := ledgerreadmodel.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(
				mtest.WriteError{
					Index:   0,
					Code:    0,
					Message: "an error",
				},
			),
		)

		eventData := ledger.NewAccountMovementRegisteredData{
			AccountMonthId:  accountMonthId,
			MovementTypeId:  movementTypeId1,
			Action:          common.Debit,
			Amount:          setupBalance(),
			Date:            date,
			SourceAccountId: nil,
			Description:     "",
			Notes:           nil,
			TagIds:          nil,
		}

		err := readModelRepository.RegisterAccountMovement(context.Background(), accountMonthId, &eventData)
		requires.Error(err)

		assertRegisterAccountMovement(common.Debit, assertEventsForUpdate(mt, asserts, requires), asserts)
	})
}

func TestReadModelRepository_GetByAccountActiveMonth(t *testing.T) {
	t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses, see account test")
}

func TestReadModelRepository_GetByAccountMonthId(t *testing.T) {
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
			asserts.Equal(ledger.AggregateType.String(), element.Value().StringValue())
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
			asserts.Equal(ledger.AggregateType.String(), element.Value().StringValue())
		case "$db":
			asserts.Equal(mongodb.DatabaseName, element.Value().StringValue())
		case "updates":
			filter := element.Value().Array().Lookup("0").Document().Lookup("q").Document()
			assertBinaryId(filter.Lookup("_id"), accountMonthId.String(), asserts)

			update = element.Value().Array().Lookup("0").Document().Lookup("u").Document()
		}
	}

	return update
}

func assertStartMonth(command bson.Raw, asserts *assert.Assertions) {
	assertBinaryId(command.Lookup("_id"), accountMonthEntity.AccountMonthId.String(), asserts)
	assertBinaryId(command.Lookup("account_id"), accountMonthEntity.AccountId.String(), asserts)

	activeMonth := command.Lookup("active_month").Document()
	asserts.Equal(int32(1), activeMonth.Lookup("month").Int32())
	asserts.Equal(int64(2023), activeMonth.Lookup("year").Int64())

	assertInt64(107060, command.Lookup("balance").Int64(), asserts)

	asserts.False(command.Lookup("month_ended").Boolean())

	asserts.Equal("{}", command.Lookup("movements").Array().String())
}

func assertEndMonth(command bson.Raw, asserts *assert.Assertions) {
	asserts.True(command.Lookup("$set").Document().Lookup("month_ended").Boolean())
}

func assertRegisterAccountMovement(movementAction common.MovementAction, command bson.Raw, asserts *assert.Assertions) {
	balanceChange := setupBalance()
	if movementAction == common.Debit {
		balanceChange = balanceChange * -1
	}

	assertInt64(
		balanceChange,
		command.Lookup("$inc").Document().Lookup("balance").Int64(),
		asserts,
	)

	movementAdded := command.Lookup("$push").Document().Lookup("movements").Document()
	assertBinaryId(movementAdded.Lookup("movement_type_id"), movementTypeId1.String(), asserts)
	assertInt64(setupBalance(), movementAdded.Lookup("amount").Int64(), asserts)
	asserts.Equal(date.UnixMilli(), movementAdded.Lookup("date").DateTime())
}

func assertBinaryId(idValue bson.RawValue, expectedId string, asserts *assert.Assertions) {
	_, data := idValue.Binary()

	actualUuid, err := googleUUID.FromBytes(data)
	asserts.NoError(err)

	asserts.Equal(expectedId, actualUuid.String())
}

func assertInt64(expected int64, actual int64, asserts *assert.Assertions) {
	asserts.Equal(expected, actual)
}
