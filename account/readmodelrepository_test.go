package account_test

import (
	"context"
	googleUUID "github.com/google/uuid"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/mongodb"
)

var expectedAccountId1 = account.Id(uuid.New())
var expectedActiveMonth = account.EntityActiveMonth{
	Month: time.August,
	Year:  2023,
}
var expectedAccountEntity1 = account.Entity{
	AccountId:           &expectedAccountId1,
	BankName:            "a bank name",
	Name:                "an account name",
	AccountType:         account.Checking,
	StartingBalance:     5069,
	StartingBalanceDate: time.Now(),
	Currency:            account.EUR,
	Notes:               "a set of notes",
	ActiveMonth:         expectedActiveMonth,
}

var accountIdBson1, _ = bson.Marshal(expectedAccountId1)
var accountBson1 = bson.D{
	{"_id", accountIdBson1},
	{"bank_name", expectedAccountEntity1.BankName},
	{"name", expectedAccountEntity1.Name},
	{"account_type", expectedAccountEntity1.AccountType},
	{"starting_balance", expectedAccountEntity1.StartingBalance},
	{"starting_balance_date", expectedAccountEntity1.StartingBalanceDate},
	{"currency", expectedAccountEntity1.Currency},
	{"notes", expectedAccountEntity1.Notes},
	{
		"active_month",
		bson.D{
			{"month", expectedAccountEntity1.ActiveMonth.Month},
			{"year", expectedAccountEntity1.ActiveMonth.Year},
		},
	},
}

var expectedAccountId2 = account.Id(uuid.New())
var expectedAccountEntity2 = account.Entity{
	AccountId:           &expectedAccountId1,
	BankName:            "another bank name",
	Name:                "annother account name",
	AccountType:         account.Savings,
	StartingBalance:     6069,
	StartingBalanceDate: time.Now().Add(1 * time.Minute),
	Currency:            account.USD,
	Notes:               "another set of notes",
	ActiveMonth: account.EntityActiveMonth{
		Month: time.April,
		Year:  2022,
	},
}

var accountIdBson2, _ = bson.Marshal(expectedAccountId2)
var accountBson2 = bson.D{
	{"_id", accountIdBson2},
	{"bank_name", expectedAccountEntity2.BankName},
	{"name", expectedAccountEntity2.Name},
	{"account_type", expectedAccountEntity2.AccountType},
	{"starting_balance", expectedAccountEntity2.StartingBalance},
	{"starting_balance_date", expectedAccountEntity2.StartingBalanceDate},
	{"currency", expectedAccountEntity2.Currency},
	{"notes", expectedAccountEntity2.Notes},
	{
		"active_month",
		bson.D{
			{"month", expectedAccountEntity2.ActiveMonth.Month},
			{"year", expectedAccountEntity2.ActiveMonth.Year},
		},
	},
}

func TestReadModelRepository_Create(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("test successful create", func(mt *mtest.T) {
		readModelRepository := account.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := readModelRepository.Create(context.Background(), expectedAccountEntity1)
		requires.NoError(err)

		update := assetUpdates(mt, asserts, requires)
		assertCreate(update, asserts)
	})

	mt.Run("test failure to create", func(mt *mtest.T) {
		readModelRepository := account.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(
				mtest.WriteError{
					Index:   0,
					Code:    0,
					Message: "an error",
				},
			),
		)

		err := readModelRepository.Create(context.Background(), expectedAccountEntity1)
		requires.Error(err)

		update := assetUpdates(mt, asserts, requires)
		assertCreate(update, asserts)
	})
}

func TestReadModelRepository_UpdateActiveMonth(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("test successful update active month", func(mt *mtest.T) {
		readModelRepository := account.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := readModelRepository.UpdateActiveMonth(context.Background(), &expectedAccountId1, expectedActiveMonth)
		requires.NoError(err)

		update := assetUpdates(mt, asserts, requires)
		assetUpdateActiveMonth(update, asserts)
	})

	mt.Run("test failure when updating active month", func(mt *mtest.T) {
		readModelRepository := account.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(
				mtest.WriteError{
					Index:   0,
					Code:    0,
					Message: "an error",
				},
			),
		)

		err := readModelRepository.UpdateActiveMonth(context.Background(), &expectedAccountId1, expectedActiveMonth)
		asserts.Error(err)

		update := assetUpdates(mt, asserts, requires)
		assetUpdateActiveMonth(update, asserts)
	})
}

func TestReadModelRepository_GetAll(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("successfully get all accounts", func(t *mtest.T) {
		t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses")

		readModelRepository := account.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		//first := mtest.CreateCursorResponse(0, "foo.bar", mtest.FirstBatch, accountBson1)
		//getMore := mtest.CreateCursorResponse(1, "foo.bar", mtest.NextBatch, accountBson2)
		//lastCursor := mtest.CreateCursorResponse(0, "foo.bar", mtest.NextBatch)
		//mt.AddMockResponses(first, getMore, lastCursor)

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "foo.bar", mtest.FirstBatch), bson.D{})

		result, err := readModelRepository.GetAll(context.Background())
		requires.NoError(err)
		asserts.Len(result, 2)

		//asserts.Equal(expectedAccountId1, result)
	})

	mt.Run("fails to get all accounts", func(t *mtest.T) {
		t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses")

		readModelRepository := account.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(
				mtest.WriteError{
					Index:   0,
					Code:    0,
					Message: "an error",
				},
			),
		)

		result, err := readModelRepository.GetAll(context.Background())
		requires.Error(err)
		asserts.Len(result, 0)
	})
}

func TestReadModelRepository_GetByAccountId(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("successfully get all accounts", func(t *mtest.T) {
		t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses")

		readModelRepository := account.NewReadModelRepository(&mongodb.MongoClient{Client: mt.Client})

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "foo.bar", mtest.FirstBatch, accountBson1))

		actualAccountEntity, err := readModelRepository.GetByAccountId(context.Background(), &expectedAccountId1)
		requires.NoError(err)

		asserts.Equal(expectedAccountEntity1, actualAccountEntity)
	})

}

func TestReadModelRepository_GetByName(t *testing.T) {
	t.Skip("runtime error: invalid memory address or nil pointer dereference on AddMockResponses")
}

func assetUpdates(mt *mtest.T, asserts *assert.Assertions, requires *require.Assertions) bson.Raw {
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
			asserts.Equal("\""+account.AggregateType.String()+"\"", element.Value().String())
		case "$db":
			asserts.Equal("\""+mongodb.DatabaseName+"\"", element.Value().String())
		case "updates":
			filter := element.Value().Array().Lookup("0").Document().Lookup("q").Document()
			assertBinaryId(filter.Lookup("_id"), asserts)

			update = element.Value().Array().Lookup("0").Document().Lookup("u").Document()
		}
	}

	return update
}

func assertCreate(update bson.Raw, asserts *assert.Assertions) {
	assertBinaryId(update.Lookup("_id"), asserts)

	asserts.Equal(expectedAccountEntity1.BankName, update.Lookup("bank_name").StringValue())
	asserts.Equal(expectedAccountEntity1.Name, update.Lookup("name").StringValue())
	asserts.Equal(expectedAccountEntity1.AccountType, account.Type(update.Lookup("account_type").Int32()))
	asserts.Equal(expectedAccountEntity1.StartingBalance, update.Lookup("starting_balance").Double())
	asserts.Equal(
		expectedAccountEntity1.StartingBalanceDate.Format("2006-02-01"),
		time.UnixMilli(update.Lookup("starting_balance_date").DateTime()).Format("2006-02-01"),
	)
	asserts.Equal(expectedAccountEntity1.Currency, account.Currency(update.Lookup("currency").StringValue()))
	asserts.Equal(expectedAccountEntity1.Notes, update.Lookup("notes").StringValue())
	assertActiveMonth(update.Lookup("active_month"), asserts)
}

func assetUpdateActiveMonth(update bson.Raw, asserts *assert.Assertions) {
	activeMonth := update.Lookup("$set").Document().Lookup("active_month")

	assertActiveMonth(activeMonth, asserts)
}

func assertBinaryId(idValue bson.RawValue, asserts *assert.Assertions) {
	_, data := idValue.Binary()

	actualUuid, err := googleUUID.FromBytes(data)
	asserts.NoError(err)

	asserts.Equal(expectedAccountId1.String(), actualUuid.String())
}

func assertActiveMonth(activeMonth bson.RawValue, asserts *assert.Assertions) {
	asserts.Equal(
		expectedActiveMonth.Month,
		time.Month(activeMonth.Document().Lookup("month").Int32()),
	)
	asserts.Equal(
		expectedActiveMonth.Year,
		uint(activeMonth.Document().Lookup("year").Int64()),
	)
}
