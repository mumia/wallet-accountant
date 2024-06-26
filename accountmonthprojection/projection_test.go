package accountmonthprojection_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/accountmonthprojection"
	"walletaccountant/accountmonthreadmodel"
	"walletaccountant/common"
	"walletaccountant/tagcategory"
)

func TestProjection_HandleEvent_NewAccountMovementRegistered(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	newMovementTypeRegisteredData := accountmonth.NewAccountMovementRegisteredData{
		AccountMonthId:  accountMonthId,
		MovementTypeId:  movementTypeId1,
		Action:          common.Debit,
		Amount:          104020,
		Date:            date,
		SourceAccountId: nil,
		Description:     "Test description",
		Notes:           nil,
		TagIds:          []*tagcategory.TagId{&tagId1},
	}

	expectedRegisterAccountMonthId := accountMonthId

	expectedRegisterMovement := accountmonthreadmodel.EntityMovement{
		MovementTypeId:  movementTypeId1,
		Action:          common.Debit,
		Amount:          104020,
		Date:            date,
		SourceAccountId: nil,
		Description:     "Test description",
		Notes:           nil,
		TagIds:          []*tagcategory.TagId{&tagId1},
	}

	registerCallCount := 0
	getByAccountMonthIdCallCount := 0
	repository := &accountmonthreadmodel.ReadModelRepositoryMock{
		RegisterAccountMovementFn: func(
			ctx context.Context,
			accountMonthId *accountmonth.Id,
			eventData *accountmonth.NewAccountMovementRegisteredData,
		) error {
			registerCallCount++

			asserts.Equal(expectedRegisterAccountMonthId, accountMonthId)
			asserts.Equal(expectedRegisterMovement.MovementTypeId, eventData.MovementTypeId)
			asserts.Equal(expectedRegisterMovement.Action, eventData.Action)
			asserts.Equal(expectedRegisterMovement.Amount, eventData.Amount)
			asserts.Equal(expectedRegisterMovement.Date, eventData.Date)
			asserts.Equal(expectedRegisterMovement.SourceAccountId, eventData.SourceAccountId)
			asserts.Equal(expectedRegisterMovement.Description, eventData.Description)
			asserts.Equal(expectedRegisterMovement.Notes, eventData.Notes)
			asserts.Equal(expectedRegisterMovement.TagIds, eventData.TagIds)

			return nil
		},
		GetByAccountMonthIdFn: func(
			ctx context.Context,
			accountMonthId *accountmonth.Id,
		) (*accountmonthreadmodel.Entity, error) {
			getByAccountMonthIdCallCount++

			return &accountMonthEntity, nil
		},
	}

	projector, err := accountmonthprojection.NewProjection(repository)
	requires.NoError(err)

	newMovementTypeRegisteredEvent := eventhorizon.NewEvent(
		accountmonth.NewAccountMovementRegistered,
		&newMovementTypeRegisteredData,
		time.Now(),
		eventhorizon.ForAggregate(accountmonth.AggregateType, *accountMonthId, 1),
	)

	err = projector.HandleEvent(context.Background(), newMovementTypeRegisteredEvent)
	requires.NoError(err)

	asserts.Equal(1, registerCallCount, "expected movement register call")
	asserts.Equal(1, getByAccountMonthIdCallCount, "expected getByAccountMonthId call")
}

func TestProjection_HandleEvent_MonthStarted(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	monthStartedData := accountmonth.MonthStartedData{
		AccountMonthId: accountMonthId,
		AccountId:      accountId1,
		StartBalance:   105000,
		Month:          month,
		Year:           year,
	}

	startMonthCallCount := 0
	getByAccountMonthIdCallCount := 0
	repository := &accountmonthreadmodel.ReadModelRepositoryMock{
		StartMonthFn: func(
			ctx context.Context,
			accountMonthId *accountmonth.Id,
			accountId *account.Id,
			startBalance int64,
			month time.Month,
			year uint,
		) error {
			startMonthCallCount++

			asserts.Equal(monthStartedData.AccountMonthId, accountMonthId)
			asserts.Equal(monthStartedData.AccountId, accountId)
			asserts.Equal(monthStartedData.StartBalance, startBalance)
			asserts.Equal(monthStartedData.Month, month)
			asserts.Equal(monthStartedData.Year, year)

			return nil
		},
		GetByAccountMonthIdFn: func(
			ctx context.Context,
			accountMonthId *accountmonth.Id,
		) (*accountmonthreadmodel.Entity, error) {
			getByAccountMonthIdCallCount++

			return nil, nil
		},
	}

	projector, err := accountmonthprojection.NewProjection(repository)
	requires.NoError(err)

	monthStartedEvent := eventhorizon.NewEvent(
		accountmonth.MonthStarted,
		&monthStartedData,
		time.Now(),
		eventhorizon.ForAggregate(accountmonth.AggregateType, *accountMonthId, 1),
	)

	err = projector.HandleEvent(context.Background(), monthStartedEvent)
	requires.NoError(err)

	asserts.Equal(1, startMonthCallCount, "expected movement register call")
	asserts.Equal(0, getByAccountMonthIdCallCount, "expected getByAccountMonthId call")
}

func TestProjection_HandleEvent_MonthEnded(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	monthEndedData := accountmonth.MonthEndedData{
		AccountMonthId: accountMonthId,
		AccountId:      accountId1,
		EndBalance:     106000,
		Month:          month,
		Year:           year,
	}

	endMonthCallCount := 0
	getByAccountMonthIdCallCount := 0
	repository := &accountmonthreadmodel.ReadModelRepositoryMock{
		EndMonthFn: func(ctx context.Context, accountMonthId *accountmonth.Id) error {
			endMonthCallCount++

			asserts.Equal(monthEndedData.AccountMonthId, accountMonthId)

			return nil
		},
		GetByAccountMonthIdFn: func(
			ctx context.Context,
			accountMonthId *accountmonth.Id,
		) (*accountmonthreadmodel.Entity, error) {
			getByAccountMonthIdCallCount++

			return nil, nil
		},
	}

	projector, err := accountmonthprojection.NewProjection(repository)
	requires.NoError(err)

	endMonthEvent := eventhorizon.NewEvent(
		accountmonth.MonthEnded,
		&monthEndedData,
		time.Now(),
		eventhorizon.ForAggregate(accountmonth.AggregateType, *accountMonthId, 1),
	)

	err = projector.HandleEvent(context.Background(), endMonthEvent)
	requires.NoError(err)

	asserts.Equal(1, endMonthCallCount, "expected movement register call")
	asserts.Equal(0, getByAccountMonthIdCallCount, "expected getByAccountMonthId call")
}
