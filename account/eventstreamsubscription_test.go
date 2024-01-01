package account_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/gofrs/uuid"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
	"walletaccountant/subscription"
)

func TestSubscribeEventStream(t *testing.T) {
	asserts := assert.New(t)
	ctx := context.Background()

	eventhorizon.RegisterEventData(
		account.NewAccountRegistered,
		func() eventhorizon.EventData { return &account.NewAccountRegisteredData{} },
	)
	defer eventhorizon.UnregisterEventData(account.NewAccountRegistered)

	var newAccountRegisteredEvent = &esdb.PersistentSubscriptionEvent{
		EventAppeared: &esdb.EventAppeared{
			Event: &esdb.ResolvedEvent{
				Link: nil,
				Event: &esdb.RecordedEvent{
					EventID:        uuid.UUID{},
					EventType:      account.NewAccountRegistered.String(),
					ContentType:    "",
					StreamID:       "",
					EventNumber:    0,
					Position:       esdb.Position{},
					CreatedDate:    time.Now(),
					Data:           []byte("{\n  \"account_id\": \"7cb19cfb-5db0-4a2f-845f-1ba2055ec341\",\n  \"bank_name\": \"N26\",\n  \"name\": \"DE N26 account\",\n  \"type\": \"checking\",\n  \"starting_balance\": 100.5,\n  \"starting_balance_date\": \"2018-08-26T00:00:00Z\",\n  \"currency\": \"EUR\",\n  \"notes\": \"These are my notes\",\n  \"active_month\": 8,\n  \"active_year\": 2018\n}"),
					SystemMetadata: nil,
					UserMetadata:   nil,
				},
				Commit: nil,
			},
			RetryCount: 0,
		},
		SubscriptionDropped: nil,
		CheckPointReached:   nil,
	}

	var subscriptionReceiveChannel = make(chan bool)
	var subscriptionReceiveCalled = 0
	persistentSubscriptionMock := &eventstoredb.PersistentSubscriptionMock{
		RecvFn: func() *esdb.PersistentSubscriptionEvent {
			var event *esdb.PersistentSubscriptionEvent

			switch subscriptionReceiveCalled {
			case 0:
				event = newAccountRegisteredEvent

			case 1:
				event = newAccountRegisteredEvent

			case 2:
				event = &esdb.PersistentSubscriptionEvent{
					SubscriptionDropped: &esdb.SubscriptionDropped{
						Error: fmt.Errorf("subscription has been dropped"),
					},
				}

			default:
				subscriptionReceiveChannel <- true

				event = &esdb.PersistentSubscriptionEvent{
					SubscriptionDropped: &esdb.SubscriptionDropped{
						Error: fmt.Errorf("subscription has been dropped"),
					},
				}
			}

			subscriptionReceiveCalled++

			return event
		},
	}

	newAccountRegisteredEventHandled := 0
	eventHandler := &mocks.EventHandlerMock{
		HandlerTypeFn: func() eventhorizon.EventHandlerType {
			return eventhorizon.EventHandlerType(account.AggregateType)
		},
		HandleEventFn: func(ctx context.Context, event eventhorizon.Event) error {
			if event.EventType() == account.NewAccountRegistered {
				newAccountRegisteredEventHandled++
			}

			if newAccountRegisteredEventHandled > 1 {
				return errors.New("should return error")
			}

			return nil
		},
	}

	subscription.SubscribeEventStreamTestHelper(
		ctx,
		t,
		account.AggregateType,
		eventhorizon.EventHandlerType(account.AggregateType),
		true,
		persistentSubscriptionMock,
		account.NewProjectionConfig(eventHandler),
		subscriptionReceiveChannel,
	)

	expectedSubscriptionReceiveCalled := 4
	asserts.Equal(
		expectedSubscriptionReceiveCalled,
		subscriptionReceiveCalled,
		fmt.Sprintf("Subscription::Recv was expected to be called %d times", expectedSubscriptionReceiveCalled),
	)
	expectedNewAccountRegisteredEventHandled := 2
	asserts.Equal(
		expectedNewAccountRegisteredEventHandled,
		newAccountRegisteredEventHandled,
		fmt.Sprintf(
			"New account registered event was not handled %d times",
			expectedNewAccountRegisteredEventHandled,
		),
	)
}
