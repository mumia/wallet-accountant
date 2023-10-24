package movementtype

import (
	"context"
	"encoding/json"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/clock"
	"walletaccountant/tagcategory"
)

func setupMovementTypeTest(instants []clock.Instant) func(id uuid.UUID) eventhorizon.Aggregate {
	accountFactory := NewFactory()
	accountFactory.clock = clock.Freeze(instants, nil)

	return accountFactory.Factory()
}

func setupMovementTypeId(withSourceAccount bool) *Id {
	uuidString := "72a196bc-d9b1-4c57-a916-3eabf1bf167b"
	if withSourceAccount {
		uuidString = "aeea307f-3c57-467c-8954-5f541aef6772"
	}

	id := Id(uuid.MustParse(uuidString))

	return &id
}

func setupMovementType(withSourceAccount bool) Type {
	if withSourceAccount {
		return Debit
	}

	return Credit
}

func setupAccountIds(withSourceAccount bool) (*account.Id, *account.Id) {
	accountId := account.Id(uuid.MustParse("f4081021-adf4-4b04-a6e5-4ad0028b96f9"))
	var sourceAccountId *account.Id
	if withSourceAccount {
		accountId = account.Id(uuid.MustParse("07a7ccde-b19c-412a-a054-bc09ac529357"))

		id := account.Id(uuid.MustParse("5c6e143d-f1a6-42ca-b9df-2f4a94628194"))
		sourceAccountId = &id
	}

	return &accountId, sourceAccountId
}

func setupDescriptionNotes(withSourceAccount bool) (string, string) {
	appendForSourceAccount := ""
	if withSourceAccount {
		appendForSourceAccount = " with source account"
	}

	return "Movement type description" + appendForSourceAccount, "My notes on my movement type" + appendForSourceAccount
}

func setupTagIds() (*tagcategory.TagId, *tagcategory.TagId) {
	tagId1 := tagcategory.TagId(uuid.MustParse("75ef2872-15ac-477f-9f2d-980596e9a761"))
	tagId2 := tagcategory.TagId(uuid.MustParse("e629d7be-725f-4af8-bdb7-0fa67211e2ba"))

	return &tagId1, &tagId2
}

func TestMovementType_HandleCommand_RegisterNewMovementType(t *testing.T) {
	instants := []clock.Instant{
		{"register movement type", time.Now()},
		{"register movement type with source account", time.Now().Add(2 * time.Minute)},
	}
	newAggregateFunc := setupMovementTypeTest(instants)

	asserts := assert.New(t)
	requires := require.New(t)

	movementTypeRegister := newAggregateFunc(*setupMovementTypeId(false)).(*MovementType)
	movementTypeWithSourceAccountRegister := newAggregateFunc(*setupMovementTypeId(true)).(*MovementType)

	command := createRegisterNewMovementTypeCommand(false)
	expectedEvent := createRegisterNewAccountEvent(false, instants[0].Instant)

	commandWithSourceAccount := createRegisterNewMovementTypeCommand(true)
	expectedEventWithSourceAccount := createRegisterNewAccountEvent(true, instants[1].Instant)

	err := movementTypeRegister.HandleCommand(context.Background(), command)
	asserts.NoError(err)

	t.Run("successfully register new movement type", func(t *testing.T) {
		uncommittedEvents := movementTypeRegister.UncommittedEvents()
		asserts.Equal(1, len(uncommittedEvents))
		asserts.Equal(expectedEvent, uncommittedEvents[0])
	})

	t.Run("fails to register new movement type, because already registered", func(t *testing.T) {
		movementTypeRegister.SetAggregateVersion(1)

		err = movementTypeRegister.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})

	t.Run("successfully register new movement type with source account", func(t *testing.T) {
		err := movementTypeWithSourceAccountRegister.HandleCommand(context.Background(), commandWithSourceAccount)
		asserts.NoError(err)

		uncommittedEvents := movementTypeWithSourceAccountRegister.UncommittedEvents()
		asserts.Equal(1, len(uncommittedEvents))

		expectedEventWithSourceAccountJson, err := json.Marshal(expectedEventWithSourceAccount)
		requires.NoError(err)
		actualEventWithSourceAccountJson, err := json.Marshal(uncommittedEvents[0])
		requires.NoError(err)

		asserts.Equal(expectedEventWithSourceAccountJson, actualEventWithSourceAccountJson)
	})
}

//func TestAccount_HandleCommand_StartNextMonth(t *testing.T) {
//	instants := []clock.Instant{
//		{"start next month", time.Now()},
//		{"start another next month", time.Now().Add(2 * time.Second)},
//	}
//	newAggregateFunc, nextMonthSameYearData, nextMonthDifferentYearData := setupMovementTypeTest(instants)
//
//	asserts := assert.New(t)
//
//	type testCase struct {
//		testName      string
//		account       *Account
//		command       eventhorizon.Command
//		expectedEvent eventhorizon.Event
//	}
//
//	accountId := uuid.New()
//
//	// for start next month same year
//	accountNextMonthSameYear := newAggregateFunc(accountId).(*Account)
//	err := accountNextMonthSameYear.ApplyEvent(
//		context.Background(),
//		createRegisterNewAccountEvent(createAccountData(Id(accountId), nil), time.Now()),
//	)
//	asserts.NoError(err)
//	accountNextMonthSameYear.SetAggregateVersion(1)
//
//	startNextMonthSameYearEvent := createStartNextMonthEvent(
//		accountNextMonthSameYear.EntityID(),
//		nextMonthSameYearData,
//		instants[0].Instant,
//		2,
//	)
//
//	// for start next month next year
//	accountNextMonthDifferentYear := newAggregateFunc(accountId).(*Account)
//	err = accountNextMonthDifferentYear.ApplyEvent(
//		context.Background(),
//		createRegisterNewAccountEvent(*accountNextMonthSameYear, time.Now()),
//	)
//	asserts.NoError(err)
//	err = accountNextMonthDifferentYear.ApplyEvent(context.Background(), startNextMonthSameYearEvent)
//	asserts.NoError(err)
//	accountNextMonthDifferentYear.SetAggregateVersion(2)
//
//	testCases := []testCase{
//		{
//			testName:      "successfully start next month staying in same year",
//			account:       accountNextMonthSameYear,
//			command:       createStartNextMonthCommand(accountNextMonthSameYear.EntityID()),
//			expectedEvent: startNextMonthSameYearEvent,
//		},
//		{
//			testName: "successfully start next month changing to next year",
//			account:  accountNextMonthDifferentYear,
//			command:  createStartNextMonthCommand(accountNextMonthDifferentYear.EntityID()),
//			expectedEvent: createStartNextMonthEvent(
//				accountNextMonthDifferentYear.EntityID(),
//				nextMonthDifferentYearData,
//				instants[1].Instant,
//				3,
//			),
//		},
//	}
//
//	for _, testCaseData := range testCases {
//		t.Run(testCaseData.testName, func(t *testing.T) {
//			err := testCaseData.account.HandleCommand(context.Background(), testCaseData.command)
//			asserts.NoError(err)
//
//			uncommittedEvents := testCaseData.account.UncommittedEvents()
//			asserts.Equal(1, len(uncommittedEvents))
//			asserts.Equal(testCaseData.expectedEvent, uncommittedEvents[0])
//		})
//	}
//
//	t.Run("fails to start next month, because account not registered", func(t *testing.T) {
//		account := newAggregateFunc(accountId).(*Account)
//
//		command := createStartNextMonthCommand(account.EntityID())
//
//		err := account.HandleCommand(context.Background(), command)
//		asserts.Error(err)
//	})
//}
//
//func TestAccount_ApplyEvent_RegisterNewAccount(t *testing.T) {
//	instants := []clock.Instant{
//		{"register account", time.Now()},
//	}
//	newAggregateFunc, _, _ := setupMovementTypeTest(instants)
//
//	asserts := assert.New(t)
//
//	t.Run("Correctly applies register new account event", func(t *testing.T) {
//		accountId := uuid.New()
//
//		account := newAggregateFunc(accountId).(*Account)
//		accountData := createAccountData(accountId, nil)
//
//		newAccountRegisteredEvent := createRegisterNewAccountEvent(accountData, instants[0].Instant)
//
//		err := account.ApplyEvent(context.Background(), newAccountRegisteredEvent)
//		asserts.NoError(err)
//
//		assetAccountValues(account, accountData, accountData.ActiveMonth(), asserts)
//	})
//}
//
//func TestAccount_ApplyEvent_StartNextMonth(t *testing.T) {
//	instants := []clock.Instant{
//		{"start next month", time.Now().Add(1 * time.Second)},
//	}
//	newAggregateFunc, nextMonth, _ := setupMovementTypeTest(instants)
//
//	asserts := assert.New(t)
//
//	t.Run("Correctly applies start next month event", func(t *testing.T) {
//		accountId := uuid.New()
//
//		account := newAggregateFunc(accountId).(*Account)
//		accountData := createAccountData(accountId, &nextMonth)
//
//		newAccountRegisteredEvent := createRegisterNewAccountEvent(accountData, time.Now())
//		startNextMonthEvent := createStartNextMonthEvent(accountId, nextMonth, time.Now(), 1)
//
//		err := account.ApplyEvent(context.Background(), newAccountRegisteredEvent)
//		asserts.NoError(err)
//		err = account.ApplyEvent(context.Background(), startNextMonthEvent)
//		asserts.NoError(err)
//
//		assetAccountValues(account, accountData, accountData.ActiveMonth(), asserts)
//	})
//}

func createRegisterNewMovementTypeCommand(withSourceAccount bool) eventhorizon.Command {
	tagId1, tagId2 := setupTagIds()

	var tagIds []*tagcategory.TagId
	if withSourceAccount {
		tagIds = []*tagcategory.TagId{tagId2}
	} else {
		tagIds = []*tagcategory.TagId{tagId1, tagId2}
	}

	accountId, sourceAccountId := setupAccountIds(withSourceAccount)
	description, notes := setupDescriptionNotes(withSourceAccount)

	return &RegisterNewMovementType{
		MovementTypeId:  *setupMovementTypeId(withSourceAccount),
		Type:            setupMovementType(withSourceAccount),
		AccountId:       *accountId,
		SourceAccountId: sourceAccountId,
		Description:     description,
		Notes:           notes,
		TagIds:          tagIds,
	}
}

func createRegisterNewAccountEvent(withSourceAccount bool, createdAt time.Time) eventhorizon.Event {
	movementTypeId := setupMovementTypeId(withSourceAccount)
	accountId, sourceAccountId := setupAccountIds(withSourceAccount)
	description, notes := setupDescriptionNotes(withSourceAccount)

	tagId1, tagId2 := setupTagIds()

	var tagIds []*tagcategory.TagId
	if withSourceAccount {
		tagIds = []*tagcategory.TagId{tagId2}
	} else {
		tagIds = []*tagcategory.TagId{tagId1, tagId2}
	}

	return eventhorizon.NewEvent(
		NewMovementTypeRegistered,
		NewMovementTypeRegisteredData{
			MovementTypeId:  movementTypeId,
			Type:            setupMovementType(withSourceAccount),
			AccountId:       accountId,
			SourceAccountId: sourceAccountId,
			Description:     description,
			Notes:           notes,
			TagIds:          tagIds,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *movementTypeId, 1),
	)
}

//func assetAccountValues(
//	account *Account,
//	expectedAccountData Account,
//	expectedActiveMonth ActiveMonth,
//	asserts *assert.Assertions,
//) {
//	asserts.Equal(expectedAccountData.EntityID(), account.EntityID())
//	asserts.Equal(expectedAccountData.BankName(), account.BankName())
//	asserts.Equal(expectedAccountData.Name(), account.Name())
//	asserts.Equal(expectedAccountData.AccountType(), account.AccountType())
//	asserts.Equal(expectedAccountData.StartingBalance(), account.StartingBalance())
//	asserts.Equal(expectedAccountData.StartingBalanceDate(), account.StartingBalanceDate())
//	asserts.Equal(expectedAccountData.Notes(), account.Notes())
//	asserts.Equal(expectedActiveMonth.Month(), account.ActiveMonth().Month())
//	asserts.Equal(expectedActiveMonth.Year(), account.ActiveMonth().Year())
//}
