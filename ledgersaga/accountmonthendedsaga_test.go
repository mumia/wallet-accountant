package ledgersaga_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	uuid2 "github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/common"
	"walletaccountant/eventstoredb"
	"walletaccountant/ledger"
	"walletaccountant/ledgersaga"
	"walletaccountant/mocks"
)

func TestAccountMonthEndedSaga_Matcher(t *testing.T) {
	sagaSubject, err := ledgersaga.NewAccountMonthEndedSaga(
		&eventstoredb.EventStoreFactoryMock{
			CreateEventStoreFn: func(aggregateType eventhorizon.AggregateType, batchSize uint64) eventhorizon.EventStore {
				return &eventstoredb.EventStoreMock{}
			},
		},
	)
	require.NoError(t, err)

	assert.Equal(
		t,
		eventhorizon.MatchEvents{
			ledger.MonthEnded,
		},
		sagaSubject.Matcher(),
	)
}

func TestAccountMonthEndedSaga_RunSaga(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	eventhorizon.RegisterAggregate(account.NewFactory().Factory())

	monthEndedData := ledger.MonthEndedData{
		AccountMonthId: accountMonthId,
		AccountId:      accountId1,
		EndBalance:     100043,
		Month:          month,
		Year:           year,
	}

	newAccountRegisteredEvent := eventhorizon.NewEvent(
		ledger.MonthEnded,
		&monthEndedData,
		time.Now(),
		eventhorizon.ForAggregate(account.AggregateType, *accountId1, 1),
	)

	handleCommandCalled := 0
	commandHandler := mocks.CommandHandlerMock{
		HandleCommandFn: func(ctx context.Context, command eventhorizon.Command) error {
			handleCommandCalled++

			var expectedCommand eventhorizon.Command
			switch handleCommandCalled {
			case 1:
				expectedCommand = &account.StartNextMonth{
					AccountId: *accountId1,
					Balance:   100043,
				}

			case 2:
				expectedCommand = &ledger.StartAccountMonth{
					AccountMonthId: *accountMonthId,
					AccountId:      *accountId1,
					StartBalance:   100043,
					Month:          month,
					Year:           year,
				}
			}

			asserts.Equal(expectedCommand, command)

			return nil
		},
	}

	notes := "my account notes"
	sagaSubject, err := ledgersaga.NewAccountMonthEndedSaga(
		&eventstoredb.EventStoreFactoryMock{
			CreateEventStoreFn: func(aggregateType eventhorizon.AggregateType, batchSize uint64) eventhorizon.EventStore {
				return &eventstoredb.EventStoreMock{
					LoadFromFn: func(ctx context.Context, uuid uuid2.UUID, version int) ([]eventhorizon.Event, error) {
						return []eventhorizon.Event{
							eventhorizon.NewEvent(
								account.NewAccountRegistered,
								&account.NewAccountRegisteredData{
									AccountId:           accountId1,
									BankName:            "bank name",
									Name:                "account name",
									AccountType:         common.Checking,
									StartingBalance:     206900,
									StartingBalanceDate: time.Now(),
									Currency:            account.USD,
									Notes:               &notes,
									ActiveMonth:         month,
									ActiveYear:          year,
								},
								time.Now(),
								eventhorizon.ForAggregate(
									account.AggregateType,
									uuid2.MustParse(accountId1.String()),
									1,
								),
							),
						}, nil
					},
				}
			},
		},
	)
	requires.NoError(err)
	err = sagaSubject.RunSaga(context.Background(), newAccountRegisteredEvent, &commandHandler)
	requires.NoError(err)

	asserts.Equal(2, handleCommandCalled)
}
