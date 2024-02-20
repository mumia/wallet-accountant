package accountprojection_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountprojection"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
)

func TestProjection_HandleEvent(t *testing.T) {
	asserts := assert.New(t)

	accountId := account.Id(uuid.New())
	notes := "my account notes"
	newAccountRegisteredData := account.NewAccountRegisteredData{
		AccountId:           &accountId,
		BankName:            "bank name",
		Name:                "account name",
		AccountType:         common.Checking,
		StartingBalance:     2069,
		StartingBalanceDate: time.Now(),
		Currency:            account.USD,
		Notes:               &notes,
		ActiveMonth:         time.December,
		ActiveYear:          2022,
	}

	expectedAccountEntity := accountreadmodel.Entity{
		AccountId:           newAccountRegisteredData.AccountId,
		BankName:            newAccountRegisteredData.BankName,
		Name:                newAccountRegisteredData.Name,
		AccountType:         newAccountRegisteredData.AccountType,
		StartingBalance:     newAccountRegisteredData.StartingBalance,
		StartingBalanceDate: newAccountRegisteredData.StartingBalanceDate,
		Currency:            newAccountRegisteredData.Currency,
		Notes:               newAccountRegisteredData.Notes,
		ActiveMonth: accountreadmodel.EntityActiveMonth{
			Month: newAccountRegisteredData.ActiveMonth,
			Year:  newAccountRegisteredData.ActiveYear,
		},
	}

	nextMonthStartedData := account.NextMonthStartedData{
		NextMonth: time.January,
		NextYear:  2023,
	}

	expectedActiveMonthEntity := accountreadmodel.EntityActiveMonth{
		Month: nextMonthStartedData.NextMonth,
		Year:  nextMonthStartedData.NextYear,
	}

	createCallCount := 0
	updateActiveMonthCallCount := 0
	repository := &accountreadmodel.ReadModelRepositoryMock{
		CreateFn: func(ctx context.Context, actualAccount accountreadmodel.Entity) error {
			createCallCount++

			asserts.Equal(expectedAccountEntity, actualAccount)

			return nil
		},
		UpdateActiveMonthFn: func(
			ctx context.Context,
			accountId *account.Id,
			activeMonth accountreadmodel.EntityActiveMonth,
		) error {
			updateActiveMonthCallCount++

			asserts.Equal(expectedActiveMonthEntity, activeMonth)

			return nil
		},
	}
	logger := zaptest.NewLogger(t)

	projector := accountprojection.NewProjection(repository, logger)

	newAccountRegisteredEvent := eventhorizon.NewEvent(
		account.NewAccountRegistered,
		&newAccountRegisteredData,
		time.Now(),
		eventhorizon.ForAggregate(account.AggregateType, accountId, 1),
	)

	ctx, cancelCtx := context.WithCancel(context.Background())

	type eventCountStruct struct {
		count int
	}

	eventCount := &eventCountStruct{0}
	go func(eventCount *eventCountStruct) {
		keepRunning := true
		for keepRunning {
			select {
			case <-ctx.Done():
				keepRunning = false

			case <-projector.UpdateChannel():
				eventCount.count++
			}
		}
	}(eventCount)

	err := projector.HandleEvent(context.Background(), newAccountRegisteredEvent)
	asserts.NoError(err)

	nextMonthStartedEvent := eventhorizon.NewEvent(
		account.NextMonthStarted,
		&nextMonthStartedData,
		time.Now(),
		eventhorizon.ForAggregate(account.AggregateType, accountId, 1),
	)

	err = projector.HandleEvent(context.Background(), nextMonthStartedEvent)
	asserts.NoError(err)

	// Wait for all update channel messages to be processed
	time.Sleep(50 * time.Millisecond)
	cancelCtx()

	asserts.Equal(1, createCallCount)
	asserts.Equal(1, updateActiveMonthCallCount)
	asserts.Equal(2, eventCount.count)
}
