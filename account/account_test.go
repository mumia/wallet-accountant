package account

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"walletaccountant/clock"
	"walletaccountant/common"
)

func setupAccountTest(instants []clock.Instant) (func(id uuid.UUID) eventhorizon.Aggregate, ActiveMonth, ActiveMonth) {
	accountFactory := NewFactory()
	accountFactory.clock = clock.Freeze(instants, nil)

	nextMonthSameYearData := ActiveMonth{
		month: time.December,
		year:  2023,
	}

	nextMonthDifferentYearData := ActiveMonth{
		month: time.January,
		year:  2024,
	}

	return accountFactory.Factory(), nextMonthSameYearData, nextMonthDifferentYearData
}

func TestAccount_HandleCommand_RegisterNewAccount(t *testing.T) {
	instants := []clock.Instant{
		{"register account", time.Now()},
	}
	newAggregateFunc, _, _ := setupAccountTest(instants)

	asserts := assert.New(t)
	accountId := uuid.New()
	startBalanceDate := time.Date(2023, time.November, 1, 0, 0, 0, 0, time.UTC)

	// for register
	accountRegister := newAggregateFunc(accountId).(*Account)

	command := createRegisterNewAccountCommand(accountId, startBalanceDate)
	expectedEvent := createRegisterNewAccountEvent(accountId, startBalanceDate, instants[0].Instant)

	err := accountRegister.HandleCommand(context.Background(), command)
	asserts.NoError(err)

	t.Run("successfully register new account", func(t *testing.T) {
		uncommittedEvents := accountRegister.UncommittedEvents()
		asserts.Equal(1, len(uncommittedEvents))
		asserts.Equal(expectedEvent, uncommittedEvents[0])
	})

	t.Run("fails to register new account, because already registered", func(t *testing.T) {
		accountRegister.SetAggregateVersion(1)

		err = accountRegister.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})
}

func TestAccount_HandleCommand_StartNextMonth(t *testing.T) {
	instants := []clock.Instant{
		{"start next month", time.Now()},
		{"start another next month", time.Now().Add(2 * time.Second)},
	}
	newAggregateFunc, nextMonthSameYearData, nextMonthDifferentYearData := setupAccountTest(instants)

	asserts := assert.New(t)

	type testCase struct {
		testName      string
		account       *Account
		command       eventhorizon.Command
		expectedEvent eventhorizon.Event
	}

	accountId := uuid.New()
	startBalanceDate := time.Date(2023, time.November, 1, 0, 0, 0, 0, time.UTC)

	// for start next month same year
	accountNextMonthSameYear := newAggregateFunc(accountId).(*Account)
	err := accountNextMonthSameYear.ApplyEvent(
		context.Background(),
		createRegisterNewAccountEvent(accountId, startBalanceDate, time.Now()),
	)
	asserts.NoError(err)
	accountNextMonthSameYear.SetAggregateVersion(1)

	startNextMonthSameYearEvent := createStartNextMonthEvent(
		accountNextMonthSameYear.EntityID(),
		100000,
		nextMonthSameYearData,
		instants[0].Instant,
		2,
	)

	// for start next month next year
	accountNextMonthDifferentYear := newAggregateFunc(accountId).(*Account)
	err = accountNextMonthDifferentYear.ApplyEvent(
		context.Background(),
		createRegisterNewAccountEvent(accountId, startBalanceDate, time.Now()),
	)
	asserts.NoError(err)
	err = accountNextMonthDifferentYear.ApplyEvent(context.Background(), startNextMonthSameYearEvent)
	asserts.NoError(err)
	accountNextMonthDifferentYear.SetAggregateVersion(2)

	testCases := []testCase{
		{
			testName: "successfully start next month staying in same year",
			account:  accountNextMonthSameYear,
			command: createStartNextMonthCommand(
				accountNextMonthSameYear.EntityID(),
				100000,
			),
			expectedEvent: startNextMonthSameYearEvent,
		},
		{
			testName: "successfully start next month changing to next year",
			account:  accountNextMonthDifferentYear,
			command: createStartNextMonthCommand(
				accountNextMonthDifferentYear.EntityID(),
				100000,
			),
			expectedEvent: createStartNextMonthEvent(
				accountNextMonthDifferentYear.EntityID(),
				100000,
				nextMonthDifferentYearData,
				instants[1].Instant,
				3,
			),
		},
	}

	for _, testCaseData := range testCases {
		t.Run(testCaseData.testName, func(t *testing.T) {
			err := testCaseData.account.HandleCommand(context.Background(), testCaseData.command)
			asserts.NoError(err)

			uncommittedEvents := testCaseData.account.UncommittedEvents()
			asserts.Equal(1, len(uncommittedEvents))
			asserts.Equal(testCaseData.expectedEvent, uncommittedEvents[0])
		})
	}

	t.Run("fails to start next month, because account not registered", func(t *testing.T) {
		account := newAggregateFunc(accountId).(*Account)

		command := createStartNextMonthCommand(account.EntityID(), 100000)

		err := account.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})
}

func TestAccount_ApplyEvent_RegisterNewAccount(t *testing.T) {
	instants := []clock.Instant{
		{"register account", time.Now()},
	}
	newAggregateFunc, _, _ := setupAccountTest(instants)

	asserts := assert.New(t)

	t.Run("Correctly applies register new account event", func(t *testing.T) {
		accountId := uuid.New()

		account := newAggregateFunc(accountId).(*Account)
		accountData := createAccountData(accountId, nil)
		startBalanceDate := time.Date(2023, time.November, 1, 0, 0, 0, 0, time.UTC)

		newAccountRegisteredEvent := createRegisterNewAccountEvent(accountId, startBalanceDate, instants[0].Instant)

		err := account.ApplyEvent(context.Background(), newAccountRegisteredEvent)
		asserts.NoError(err)

		assetAccountValues(account, accountData, accountData.ActiveMonth(), asserts)
	})
}

func TestAccount_ApplyEvent_StartNextMonth(t *testing.T) {
	instants := []clock.Instant{
		{"start next month", time.Now().Add(1 * time.Second)},
	}
	newAggregateFunc, nextMonth, _ := setupAccountTest(instants)

	asserts := assert.New(t)

	t.Run("Correctly applies start next month event", func(t *testing.T) {
		accountId := uuid.New()

		account := newAggregateFunc(accountId).(*Account)
		accountData := createAccountData(accountId, &nextMonth)
		startBalanceDate := time.Date(2023, time.November, 1, 0, 0, 0, 0, time.UTC)

		newAccountRegisteredEvent := createRegisterNewAccountEvent(accountId, startBalanceDate, time.Now())
		startNextMonthEvent := createStartNextMonthEvent(accountId, 100000, nextMonth, time.Now(), 1)

		err := account.ApplyEvent(context.Background(), newAccountRegisteredEvent)
		asserts.NoError(err)
		err = account.ApplyEvent(context.Background(), startNextMonthEvent)
		asserts.NoError(err)

		assetAccountValues(account, accountData, accountData.ActiveMonth(), asserts)
	})
}

func createAccountData(accountId uuid.UUID, activeMonth *ActiveMonth) Account {
	startBalanceDate := time.Date(2023, time.November, 1, 0, 0, 0, 0, time.UTC)

	if activeMonth == nil {
		activeMonth = &ActiveMonth{
			month: startBalanceDate.Month(),
			year:  uint(startBalanceDate.Year()),
		}
	}

	return Account{
		AggregateBase: events.NewAggregateBase(AggregateType, accountId),
		activeMonth:   *activeMonth,
	}
}

func createRegisterNewAccountCommand(accountId Id, startBalanceDate time.Time) eventhorizon.Command {
	return &RegisterNewAccount{
		AccountId:           accountId,
		BankName:            "My Bank",
		Name:                "Account name",
		AccountType:         common.Savings,
		StartingBalance:     106900,
		StartingBalanceDate: startBalanceDate,
		Currency:            USD,
	}
}

func createStartNextMonthCommand(accountId uuid.UUID, balance int64) eventhorizon.Command {
	return &StartNextMonth{AccountId: accountId, Balance: balance}
}

func createRegisterNewAccountEvent(accountId Id, startingBalanceDate time.Time, createdAt time.Time) eventhorizon.Event {
	return eventhorizon.NewEvent(
		NewAccountRegistered,
		&NewAccountRegisteredData{
			AccountId:           &accountId,
			BankName:            "My Bank",
			Name:                "Account name",
			AccountType:         common.Savings,
			StartingBalance:     106900,
			StartingBalanceDate: startingBalanceDate,
			Currency:            USD,
			ActiveMonth:         startingBalanceDate.Month(),
			ActiveYear:          uint(startingBalanceDate.Year()),
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, accountId, 1),
	)
}

func createStartNextMonthEvent(
	aggregateId uuid.UUID,
	balance int64,
	activeMonth ActiveMonth,
	createdAt time.Time,
	version int,
) eventhorizon.Event {
	accountId := IdFromUUID(aggregateId)

	return eventhorizon.NewEvent(
		NextMonthStarted,
		&NextMonthStartedData{
			AccountId: accountId,
			Balance:   balance,
			NextMonth: activeMonth.Month(),
			NextYear:  activeMonth.Year(),
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, aggregateId, version),
	)
}

func assetAccountValues(
	account *Account,
	expectedAccountData Account,
	expectedActiveMonth ActiveMonth,
	asserts *assert.Assertions,
) {
	asserts.Equal(expectedAccountData.EntityID(), account.EntityID())
	asserts.Equal(expectedActiveMonth.Month(), account.ActiveMonth().Month())
	asserts.Equal(expectedActiveMonth.Year(), account.ActiveMonth().Year())
}
