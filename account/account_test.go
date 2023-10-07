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

	// for register
	accountRegister := newAggregateFunc(accountId).(*Account)
	accountDataRegister := createAccountData(Id(accountRegister.EntityID()), nil)

	command := createRegisterNewAccountCommand(accountDataRegister)
	expectedEvent := createRegisterNewAccountEvent(accountDataRegister, instants[0].Instant)

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

	// for start next month same year
	accountNextMonthSameYear := newAggregateFunc(accountId).(*Account)
	err := accountNextMonthSameYear.ApplyEvent(
		context.Background(),
		createRegisterNewAccountEvent(createAccountData(Id(accountId), nil), time.Now()),
	)
	asserts.NoError(err)
	accountNextMonthSameYear.SetAggregateVersion(1)

	startNextMonthSameYearEvent := createStartNextMonthEvent(
		accountNextMonthSameYear.EntityID(),
		nextMonthSameYearData,
		instants[0].Instant,
		2,
	)

	// for start next month next year
	accountNextMonthDifferentYear := newAggregateFunc(accountId).(*Account)
	err = accountNextMonthDifferentYear.ApplyEvent(
		context.Background(),
		createRegisterNewAccountEvent(*accountNextMonthSameYear, time.Now()),
	)
	asserts.NoError(err)
	err = accountNextMonthDifferentYear.ApplyEvent(context.Background(), startNextMonthSameYearEvent)
	asserts.NoError(err)
	accountNextMonthDifferentYear.SetAggregateVersion(2)

	testCases := []testCase{
		{
			testName:      "successfully start next month staying in same year",
			account:       accountNextMonthSameYear,
			command:       createStartNextMonthCommand(accountNextMonthSameYear.EntityID()),
			expectedEvent: startNextMonthSameYearEvent,
		},
		{
			testName: "successfully start next month changing to next year",
			account:  accountNextMonthDifferentYear,
			command:  createStartNextMonthCommand(accountNextMonthDifferentYear.EntityID()),
			expectedEvent: createStartNextMonthEvent(
				accountNextMonthDifferentYear.EntityID(),
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

		command := createStartNextMonthCommand(account.EntityID())

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

		newAccountRegisteredEvent := createRegisterNewAccountEvent(accountData, instants[0].Instant)

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

		newAccountRegisteredEvent := createRegisterNewAccountEvent(accountData, time.Now())
		startNextMonthEvent := createStartNextMonthEvent(accountId, nextMonth, time.Now(), 1)

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
		AggregateBase:       events.NewAggregateBase(AggregateType, accountId),
		bankName:            "My Bank",
		name:                "Account name",
		accountType:         Savings,
		startingBalance:     1069,
		startingBalanceDate: startBalanceDate,
		currency:            USD,
		notes:               "My notes on my account",
		activeMonth: ActiveMonth{
			month: activeMonth.Month(),
			year:  activeMonth.Year(),
		},
	}
}

func createRegisterNewAccountCommand(accountData Account) eventhorizon.Command {
	return &RegisterNewAccount{
		AccountId:           accountData.EntityID(),
		BankName:            accountData.BankName(),
		Name:                accountData.Name(),
		AccountType:         accountData.AccountType(),
		StartingBalance:     accountData.StartingBalance(),
		StartingBalanceDate: accountData.StartingBalanceDate(),
		Currency:            accountData.Currency(),
		Notes:               accountData.Notes(),
	}
}

func createStartNextMonthCommand(accountId uuid.UUID) eventhorizon.Command {
	return &StartNextMonth{AccountId: accountId}
}

func createRegisterNewAccountEvent(accountData Account, createdAt time.Time) eventhorizon.Event {
	accountId := Id(accountData.EntityID())
	return eventhorizon.NewEvent(
		NewAccountRegistered,
		NewAccountRegisteredData{
			AccountID:           &accountId,
			BankName:            accountData.BankName(),
			Name:                accountData.Name(),
			AccountType:         accountData.AccountType(),
			StartingBalance:     accountData.StartingBalance(),
			StartingBalanceDate: accountData.StartingBalanceDate(),
			Currency:            accountData.Currency(),
			Notes:               accountData.Notes(),
			ActiveMonth:         accountData.StartingBalanceDate().Month(),
			ActiveYear:          uint(accountData.StartingBalanceDate().Year()),
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, accountId, 1),
	)
}

func createStartNextMonthEvent(
	aggregateId uuid.UUID,
	activeMonth ActiveMonth,
	createdAt time.Time,
	version int,
) eventhorizon.Event {
	return eventhorizon.NewEvent(
		NextMonthStarted,
		NextMonthStartedData{
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
	asserts.Equal(expectedAccountData.BankName(), account.BankName())
	asserts.Equal(expectedAccountData.Name(), account.Name())
	asserts.Equal(expectedAccountData.AccountType(), account.AccountType())
	asserts.Equal(expectedAccountData.StartingBalance(), account.StartingBalance())
	asserts.Equal(expectedAccountData.StartingBalanceDate(), account.StartingBalanceDate())
	asserts.Equal(expectedAccountData.Notes(), account.Notes())
	asserts.Equal(expectedActiveMonth.Month(), account.ActiveMonth().Month())
	asserts.Equal(expectedActiveMonth.Year(), account.ActiveMonth().Year())
}
