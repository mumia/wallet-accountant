package ledger

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
	"walletaccountant/common"
	"walletaccountant/tagcategory"
)

func setupAccountMonthTest(instants []clock.Instant) func(id uuid.UUID) eventhorizon.Aggregate {
	factory := NewFactory()
	factory.clock = clock.Freeze(instants, nil)

	return factory.Factory()
}

func setupAccountMonthId() *Id {
	uuidString := "6c686c88-3f90-494f-bbb4-9c412d514302"

	return IdFromUUIDString(uuidString)
}

func setupAccountMovementId() *AccountMovementId {
	uuidString := "bbbcfa83-d879-4c24-b77d-a44e8ee572b2"

	return AccountMovementIdFromUUIDString(uuidString)
}

func setupMovementTypeId() *Id {
	uuidString := "72a196bc-d9b1-4c57-a916-3eabf1bf167b"

	return IdFromUUIDString(uuidString)
}

func setupAccountId() *account.Id {
	return account.IdFromUUIDString("f4081021-adf4-4b04-a6e5-4ad0028b96f9")
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

	id1, err := IdGenerate(accountId, month, year)
	requires.NoError(err)

	id2, err := IdGenerate(accountId, month, year)
	requires.NoError(err)

	id3, err := IdGenerate(accountId, otherMonth, year)
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
		asserts.Equal(expectedEvent.EventType(), uncommittedEvents[0].EventType())
		asserts.Equal(expectedEvent.AggregateType(), uncommittedEvents[0].AggregateType())
		asserts.Equal(expectedEvent.Data(), uncommittedEvents[0].Data())
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
		accountMonthAggregate.balance = 101000
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
	tagId := tagcategory.TagIdFromUUIDString("84c34932-9d22-40f0-9e56-443fcafc84fe")

	return &RegisterNewAccountMovement{
		AccountMonthId:    *setupAccountMonthId(),
		AccountMovementId: *setupAccountMovementId(),
		MovementTypeId:    setupMovementTypeId(),
		Action:            common.Debit,
		Amount:            103200,
		Date:              date,
		SourceAccountId:   nil,
		Description:       "Movement description",
		Notes:             nil,
		TagIds:            []*tagcategory.TagId{tagId},
	}
}

func createStartAccountMonthCommand() eventhorizon.Command {
	month, year := setupActiveMonth(nil)

	return &StartAccountMonth{
		AccountMonthId: *setupAccountMonthId(),
		AccountId:      *setupAccountId(),
		StartBalance:   100000,
		Month:          month,
		Year:           year,
	}
}

func createEndAccountMonthCommand() eventhorizon.Command {
	month, year := setupActiveMonth(nil)

	return &EndAccountMonth{
		AccountMonthId: *setupAccountMonthId(),
		AccountId:      *setupAccountId(),
		EndBalance:     101000,
		Month:          month,
		Year:           year,
	}
}

func createRegisterNewAccountEvent(date time.Time, createdAt time.Time) eventhorizon.Event {
	tagId := tagcategory.TagIdFromUUIDString("84c34932-9d22-40f0-9e56-443fcafc84fe")

	return eventhorizon.NewEvent(
		NewAccountMovementRegistered,
		&NewAccountMovementRegisteredData{
			AccountMonthId:    setupAccountMonthId(),
			AccountMovementId: setupAccountMovementId(),
			MovementTypeId:    setupMovementTypeId(),
			Action:            common.Debit,
			Amount:            103200,
			Date:              date,
			SourceAccountId:   nil,
			Description:       "Movement description",
			Notes:             nil,
			TagIds:            []*tagcategory.TagId{tagId},
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupAccountMonthId(), 2),
	)
}

func createAccountMonthStartedEvent(createdAt time.Time) eventhorizon.Event {
	month, year := setupActiveMonth(nil)

	return eventhorizon.NewEvent(
		MonthStarted,
		&MonthStartedData{
			AccountMonthId: setupAccountMonthId(),
			AccountId:      setupAccountId(),
			StartBalance:   100000,
			Month:          month,
			Year:           year,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupAccountMonthId(), 1),
	)
}

func createAccountMonthEndedEvent(createdAt time.Time) eventhorizon.Event {
	month, year := setupActiveMonth(nil)

	return eventhorizon.NewEvent(
		MonthEnded,
		&MonthEndedData{
			AccountMonthId: setupAccountMonthId(),
			AccountId:      setupAccountId(),
			EndBalance:     101000,
			Month:          month,
			Year:           year,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, *setupAccountMonthId(), 3),
	)
}
