package saga_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	saga2 "walletaccountant/account/saga"
	"walletaccountant/accountmonth"
	"walletaccountant/common"
	"walletaccountant/mocks"
	"walletaccountant/saga"
)

func TestAccountRegisterSaga_Matcher(t *testing.T) {
	sagaSubject := saga2.NewAccountRegisterSaga()

	assert.Equal(
		t,
		eventhorizon.MatchEvents{
			account.NewAccountRegistered,
		},
		sagaSubject.Matcher(),
	)
}

func TestAccountRegisterSaga_RunSaga(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	notes := "my account notes"
	newAccountRegisteredData := account.NewAccountRegisteredData{
		AccountId:           &saga.accountId1,
		BankName:            "bank name",
		Name:                "account name",
		AccountType:         common.Checking,
		StartingBalance:     2069.96,
		StartingBalanceDate: saga.date,
		Currency:            account.USD,
		Notes:               &notes,
		ActiveMonth:         saga.month,
		ActiveYear:          saga.year,
	}

	newAccountRegisteredEvent := eventhorizon.NewEvent(
		account.NewAccountRegistered,
		&newAccountRegisteredData,
		time.Now(),
		eventhorizon.ForAggregate(account.AggregateType, saga.accountId1, 1),
	)

	handleCommandCalled := 0
	commandHandler := mocks.CommandHandlerMock{
		HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
			handleCommandCalled++

			expectedCommand := &accountmonth.StartAccountMonth{
				AccountMonthId: saga.accountMonthId,
				AccountId:      saga.accountId1,
				StartBalance:   2069.96,
				Month:          saga.month,
				Year:           saga.year,
			}

			asserts.Equal(expectedCommand, command)

			return nil
		},
	}

	sagaSubject := saga2.NewAccountRegisterSaga()
	err := sagaSubject.RunSaga(context.Background(), newAccountRegisteredEvent, &commandHandler)
	requires.NoError(err)

	asserts.Equal(1, handleCommandCalled)
}
