package ledgerprojection_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/common"
	"walletaccountant/ledger"
	"walletaccountant/ledgerprojection"
	"walletaccountant/ledgerreadmodel"
	"walletaccountant/tagcategory"
)

func TestProjection_HandleEvent_NewAccountMovementRegistered(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	newMovementTypeRegisteredData := ledger.NewAccountMovementRegisteredData{
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

	expectedRegisterMovement := ledgerreadmodel.EntityMovement{
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
	repository := &ledgerreadmodel.ReadModelRepositoryMock{
		RegisterAccountMovementFn: func(
			ctx context.Context,
			accountMonthId *ledger.Id,
			eventData *ledger.NewAccountMovementRegisteredData,
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
			accountMonthId *ledger.Id,
		) (*ledgerreadmodel.Entity, error) {
			getByAccountMonthIdCallCount++

			return &accountMonthEntity, nil
		},
	}

	projector, err := ledgerprojection.NewProjection(repository)
	requires.NoError(err)

	newMovementTypeRegisteredEvent := eventhorizon.NewEvent(
		ledger.NewAccountMovementRegistered,
		&newMovementTypeRegisteredData,
		time.Now(),
		eventhorizon.ForAggregate(ledger.AggregateType, *accountMonthId, 1),
	)

	err = projector.HandleEvent(context.Background(), newMovementTypeRegisteredEvent)
	requires.NoError(err)

	asserts.Equal(1, registerCallCount, "expected movement register call")
	asserts.Equal(1, getByAccountMonthIdCallCount, "expected getByAccountMonthId call")
}

func TestProjection_HandleEvent_MonthStarted(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	monthStartedData := ledger.MonthStartedData{
		AccountMonthId: accountMonthId,
		AccountId:      accountId1,
		StartBalance:   105000,
		Month:          month,
		Year:           year,
	}

	startMonthCallCount := 0
	getByAccountMonthIdCallCount := 0
	repository := &ledgerreadmodel.ReadModelRepositoryMock{
		StartMonthFn: func(
			ctx context.Context,
			accountMonthId *ledger.Id,
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
			accountMonthId *ledger.Id,
		) (*ledgerreadmodel.Entity, error) {
			getByAccountMonthIdCallCount++

			return nil, nil
		},
	}

	projector, err := ledgerprojection.NewProjection(repository)
	requires.NoError(err)

	monthStartedEvent := eventhorizon.NewEvent(
		ledger.MonthStarted,
		&monthStartedData,
		time.Now(),
		eventhorizon.ForAggregate(ledger.AggregateType, *accountMonthId, 1),
	)

	err = projector.HandleEvent(context.Background(), monthStartedEvent)
	requires.NoError(err)

	asserts.Equal(1, startMonthCallCount, "expected movement register call")
	asserts.Equal(0, getByAccountMonthIdCallCount, "expected getByAccountMonthId call")
}

func TestProjection_HandleEvent_MonthEnded(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	monthEndedData := ledger.MonthEndedData{
		AccountMonthId: accountMonthId,
		AccountId:      accountId1,
		EndBalance:     106000,
		Month:          month,
		Year:           year,
	}

	endMonthCallCount := 0
	getByAccountMonthIdCallCount := 0
	repository := &ledgerreadmodel.ReadModelRepositoryMock{
		EndMonthFn: func(ctx context.Context, accountMonthId *ledger.Id) error {
			endMonthCallCount++

			asserts.Equal(monthEndedData.AccountMonthId, accountMonthId)

			return nil
		},
		GetByAccountMonthIdFn: func(
			ctx context.Context,
			accountMonthId *ledger.Id,
		) (*ledgerreadmodel.Entity, error) {
			getByAccountMonthIdCallCount++

			return nil, nil
		},
	}

	projector, err := ledgerprojection.NewProjection(repository)
	requires.NoError(err)

	endMonthEvent := eventhorizon.NewEvent(
		ledger.MonthEnded,
		&monthEndedData,
		time.Now(),
		eventhorizon.ForAggregate(ledger.AggregateType, *accountMonthId, 1),
	)

	err = projector.HandleEvent(context.Background(), endMonthEvent)
	requires.NoError(err)

	asserts.Equal(1, endMonthCallCount, "expected movement register call")
	asserts.Equal(0, getByAccountMonthIdCallCount, "expected getByAccountMonthId call")
}
