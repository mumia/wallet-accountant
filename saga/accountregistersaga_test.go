package saga_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/mocks"
	"walletaccountant/saga"
)

func TestAccountRegisterSaga_Matcher(t *testing.T) {
	sagaSubject := saga.NewAccountRegisterSaga()

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

	newAccountRegisteredData := account.NewAccountRegisteredData{
		AccountId:           &accountId1,
		BankName:            "bank name",
		Name:                "account name",
		AccountType:         account.Checking,
		StartingBalance:     2069.96,
		StartingBalanceDate: date,
		Currency:            account.USD,
		Notes:               "my account notes",
		ActiveMonth:         month,
		ActiveYear:          year,
	}

	newAccountRegisteredEvent := eventhorizon.NewEvent(
		account.NewAccountRegistered,
		&newAccountRegisteredData,
		time.Now(),
		eventhorizon.ForAggregate(account.AggregateType, accountId1, 1),
	)

	handleCommandCalled := 0
	commandHandler := mocks.CommandHandlerMock{
		HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
			handleCommandCalled++

			expectedCommand := &accountmonth.StartAccountMonth{
				AccountMonthId: accountMonthId,
				AccountId:      accountId1,
				StartBalance:   2069.96,
				Month:          month,
				Year:           year,
			}

			asserts.Equal(expectedCommand, command)

			return nil
		},
	}

	sagaSubject := saga.NewAccountRegisterSaga()
	err := sagaSubject.RunSaga(context.Background(), newAccountRegisteredEvent, &commandHandler)
	requires.NoError(err)

	asserts.Equal(1, handleCommandCalled)
}
