package accountmonth

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/clock"
	"walletaccountant/movementtype"
)

func setupAccountMonthTest(instants []clock.Instant) func(id uuid.UUID) eventhorizon.Aggregate {
	factory := NewFactory()
	factory.clock = clock.Freeze(instants, nil)

	return factory.Factory()
}

func setupAccountMonthId() *Id {
	uuidString := "72a196bc-d9b1-4c57-a916-3eabf1bf167b"

	id := Id(uuid.MustParse(uuidString))

	return &id
}

func setupMovementTypeId() *Id {
	uuidString := "72a196bc-d9b1-4c57-a916-3eabf1bf167b"

	id := Id(uuid.MustParse(uuidString))

	return &id
}

func setupAccountId() *account.Id {
	accountId := account.Id(uuid.MustParse("f4081021-adf4-4b04-a6e5-4ad0028b96f9"))

	return &accountId
}

func setupActiveMonth(addMonth *int) (time.Month, uint) {
	monthAdd := 0
	if addMonth != nil {
		monthAdd = *addMonth
	}

	return time.Month(int(time.February) + monthAdd), 2023
}

func TestGenerateAccountMonthId(t *testing.T) {
	t.Parallel()

	accountId := setupAccountId()
	month, year := setupActiveMonth(nil)
	addMonth := 1
	otherMonth, _ := setupActiveMonth(&addMonth)

	expectedUUID := "14bed6fd-4fbf-597b-dcc2-49d528dfc4de"

	asserts := assert.New(t)
	requires := require.New(t)

	id1, err := GenerateAccountMonthId(accountId, month, year)
	requires.NoError(err)

	id2, err := GenerateAccountMonthId(accountId, month, year)
	requires.NoError(err)

	id3, err := GenerateAccountMonthId(accountId, otherMonth, year)
	requires.NoError(err)

	requires.NotEqual(month, otherMonth)

	asserts.Equal(expectedUUID, id1.String())
	asserts.Equal(expectedUUID, id2.String())
	asserts.Equal(id1, id2)

	asserts.NotEqual(expectedUUID, id3.String())
	asserts.NotEqual(id1, id3)
}

func TestAccountMonth_HandleCommand_RegisterNewAccountMovement(t *testing.T) {
	instants := []clock.Instant{
		{"register new account movement", time.Now()},
	}
	newAggregateFunc := setupAccountMonthTest(instants)

	asserts := assert.New(t)
	requires := require.New(t)

	month, year := setupActiveMonth(nil)

	accountMonthAggregate := newAggregateFunc(*setupAccountMonthId()).(*AccountMonth)
	accountMonthAggregate.activeMonth = &ActiveMonth{
		month: month,
		year:  year,
	}
	accountMonthAggregate.SetAggregateVersion(1)

	date := time.Date(int(year), month, 1, 0, 0, 0, 0, time.UTC)
	command := createRegisterNewAccountMovementCommand(date)
	expectedEvent := createRegisterNewAccountEvent(date, instants[0].Instant)

	t.Run("successfully register new account movement", func(t *testing.T) {
		err := accountMonthAggregate.HandleCommand(context.Background(), command)
		requires.NoError(err)

		uncommittedEvents := accountMonthAggregate.UncommittedEvents()
		asserts.Equal(1, len(uncommittedEvents))
		asserts.Equal(expectedEvent, uncommittedEvents[0])
	})

	t.Run("fails to register new account movement, because account month not started", func(t *testing.T) {
		accountMonthAggregate.SetAggregateVersion(0)

		err := accountMonthAggregate.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})

	t.Run("fails to register new account movement, because active year is different", func(t *testing.T) {
		date := time.Date(int(year)+1, month, 1, 0, 0, 0, 0, time.UTC)
		command := createRegisterNewAccountMovementCommand(date)
		accountMonthAggregate.SetAggregateVersion(1)

		err := accountMonthAggregate.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})
}

func TestAccountMonth_HandleCommand_StartAccountMonth(t *testing.T) {
	instants := []clock.Instant{
		{"start account month", time.Now()},
	}
	newAggregateFunc := setupAccountMonthTest(instants)

	asserts := assert.New(t)
	requires := require.New(t)

	accountMonthAggregate := newAggregateFunc(*setupAccountMonthId()).(*AccountMonth)

	command := createStartAccountMonthCommand()
	expectedEvent := createAccountMonthStartedEvent(instants[0].Instant)

	t.Run("successfully start account month", func(t *testing.T) {
		err := accountMonthAggregate.HandleCommand(context.Background(), command)
		requires.NoError(err)

		uncommittedEvents := accountMonthAggregate.UncommittedEvents()
		asserts.Equal(1, len(uncommittedEvents))
		asserts.Equal(expectedEvent, uncommittedEvents[0])
	})

	t.Run("fails to start account month, because account month already started", func(t *testing.T) {
		accountMonthAggregate.SetAggregateVersion(1)

		err := accountMonthAggregate.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})
}

func TestAccountMonth_HandleCommand_EndAccountMonth(t *testing.T) {
	instants := []clock.Instant{
		{"end account month", time.Now()},
	}
	newAggregateFunc := setupAccountMonthTest(instants)

	asserts := assert.New(t)
	requires := require.New(t)

	month, year := setupActiveMonth(nil)

	accountMonthAggregate := newAggregateFunc(*setupAccountMonthId()).(*AccountMonth)
	accountMonthAggregate.activeMonth = &ActiveMonth{
		month: month,
		year:  year,
	}
	command := createEndAccountMonthCommand()
	expectedEvent := createAccountMonthEndedEvent(instants[0].Instant)

	t.Run("successfully end account month", func(t *testing.T) {
		accountMonthAggregate.balance = 1010
		accountMonthAggregate.SetAggregateVersion(2)

		err := accountMonthAggregate.HandleCommand(context.Background(), command)
		requires.NoError(err)

		uncommittedEvents := accountMonthAggregate.UncommittedEvents()
		asserts.Equal(1, len(uncommittedEvents))
		asserts.Equal(expectedEvent, uncommittedEvents[0])
	})

	t.Run("fails to end account month, because account month not started", func(t *testing.T) {
		accountMonthAggregate.SetAggregateVersion(0)

		err := accountMonthAggregate.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})

	t.Run("fails to end account month, because active year is different", func(t *testing.T) {
		date := time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC)
		command := createRegisterNewAccountMovementCommand(date)
		accountMonthAggregate.SetAggregateVersion(2)

		err := accountMonthAggregate.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})
}

func createRegisterNewAccountMovementCommand(date time.Time) eventhorizon.Command {
	return &RegisterNewAccountMovement{
		AccountMonthId:   *setupAccountMonthId(),
		MovementTypeId:   *setupMovementTypeId(),
		MovementTypeType: movementtype.Debit,
		Amount:           1032,
		Date:             date,
	}
}

func createStartAccountMonthCommand() eventhorizon.Command {
	month, year := setupActiveMonth(nil)

	return &StartAccountMonth{
		AccountMonthId: *setupAccountMonthId(),
		AccountId:      *setupAccountId(),
		StartBalance:   1000,
		Month:          month,
		Year:           year,
	}
}

func createEndAccountMonthCommand() eventhorizon.Command {
	month, year := setupActiveMonth(nil)

	return &EndAccountMonth{
		AccountMonthId: *setupAccountMonthId(),
		AccountId:      *setupAccountId(),
		EndBalance:     1010,
		Month:          month,
		Year:           year,
	}
}

func createRegisterNewAccountEvent(date time.Time, createdAt time.Time) eventhorizon.Event {
	return eventhorizon.NewEvent(
		NewAccountMovementRegistered,
		&NewAccountMovementRegisteredData{
			AccountMonthId:   setupAccountMonthId(),
			MovementTypeId:   setupMovementTypeId(),
			MovementTypeType: movementtype.Debit,
			Amount:           1032,
			Date:             date,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupMovementTypeId(), 2),
	)
}

func createAccountMonthStartedEvent(createdAt time.Time) eventhorizon.Event {
	month, year := setupActiveMonth(nil)

	return eventhorizon.NewEvent(
		MonthStarted,
		&MonthStartedData{
			AccountMonthId: setupAccountMonthId(),
			AccountId:      setupAccountId(),
			StartBalance:   1000,
			Month:          month,
			Year:           year,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupMovementTypeId(), 1),
	)
}

func createAccountMonthEndedEvent(createdAt time.Time) eventhorizon.Event {
	month, year := setupActiveMonth(nil)

	return eventhorizon.NewEvent(
		MonthEnded,
		&MonthEndedData{
			AccountMonthId: setupAccountMonthId(),
			AccountId:      setupAccountId(),
			EndBalance:     1010,
			Month:          month,
			Year:           year,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupMovementTypeId(), 3),
	)
}
