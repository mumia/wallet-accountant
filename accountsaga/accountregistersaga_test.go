package accountsaga_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountsaga"
	"walletaccountant/common"
	"walletaccountant/ledger"
	"walletaccountant/mocks"
)

var month = time.January
var year = uint(2023)
var date = time.Date(int(year), month, 1, 0, 0, 0, 0, time.UTC)
var accountMonthUUIDString = "46e18992-7977-9f44-4fee-b192d8c5a746"
var accountMonthId = ledger.IdFromUUIDString(accountMonthUUIDString)
var accountId1 = account.IdFromUUIDString("aeea307f-3c57-467c-8954-5f541aef6772")

func TestAccountRegisterSaga_Matcher(t *testing.T) {
	sagaSubject := accountsaga.NewAccountRegisterSaga()

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
		AccountId:           accountId1,
		BankName:            "bank name",
		Name:                "account name",
		AccountType:         common.Checking,
		StartingBalance:     206996,
		StartingBalanceDate: date,
		Currency:            account.USD,
		Notes:               &notes,
		ActiveMonth:         month,
		ActiveYear:          year,
	}

	newAccountRegisteredEvent := eventhorizon.NewEvent(
		account.NewAccountRegistered,
		&newAccountRegisteredData,
		time.Now(),
		eventhorizon.ForAggregate(account.AggregateType, *accountId1, 1),
	)

	handleCommandCalled := 0
	commandHandler := mocks.CommandHandlerMock{
		HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
			handleCommandCalled++

			expectedCommand := &ledger.StartAccountMonth{
				AccountMonthId: *accountMonthId,
				AccountId:      *accountId1,
				StartBalance:   206996,
				Month:          month,
				Year:           year,
			}

			asserts.Equal(expectedCommand, command)

			return nil
		},
	}

	sagaSubject := accountsaga.NewAccountRegisterSaga()
	err := sagaSubject.RunSaga(context.Background(), newAccountRegisteredEvent, &commandHandler)
	requires.NoError(err)

	asserts.Equal(1, handleCommandCalled)
}
