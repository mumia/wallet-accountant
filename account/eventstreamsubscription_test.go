package account_test

import (
	"context"
	"fmt"
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/gofrs/uuid"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap/zaptest"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
	"walletaccountant/projector"
)

var projectionStream = fmt.Sprintf("$ce-%s", account.AggregateType)
var subscriptionGroup = fmt.Sprintf("subscription-group-%s", account.AggregateType)

func TestSubscribeEventStream(t *testing.T) {
	asserts := assert.New(t)
	ctx := context.Background()

	lifecycle := fxtest.NewLifecycle(t)

	eventhorizon.RegisterEventData(
		account.NewAccountRegistered,
		func() eventhorizon.EventData { return &account.NewAccountRegisteredData{} },
	)

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
					Data:           []byte("{\n  \"account_id\": \"7cb19cfb-5db0-4a2f-845f-1ba2055ec341\",\n  \"bank_name\": \"N26\",\n  \"name\": \"DE N26 account\",\n  \"type\": 1,\n  \"starting_balance\": 100.5,\n  \"starting_balance_date\": \"2018-08-26T00:00:00Z\",\n  \"currency\": \"EUR\",\n  \"notes\": \"These are my notes\",\n  \"active_month\": 8,\n  \"active_year\": 2018\n}"),
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
	var subscriptionCreateCalled = false
	var subscriptionReceiveCalled = 0
	var subscriptionEventAck = false
	var subscriptionEventNack = false
	client := &eventstoredb.ClientMock{
		SubscribeToPersistentSubscriptionFn: func(
			ctx context.Context,
			streamName string,
			groupName string,
			options esdb.SubscribeToPersistentSubscriptionOptions,
		) (eventstoredb.PersistentSubscriptioner, error) {
			return &eventstoredb.PersistentSubscriptionMock{
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
				AckFn: func(messages ...*esdb.ResolvedEvent) error {
					subscriptionEventAck = true

					return nil
				},
				NackFn: func(reason string, action esdb.NackAction, messages ...*esdb.ResolvedEvent) error {
					subscriptionEventNack = true

					return nil
				},
			}, nil
		},

		CreatePersistentSubscriptionFn: func(
			ctx context.Context,
			streamName string,
			groupName string,
			options esdb.PersistentStreamSubscriptionOptions,
		) error {
			subscriptionCreateCalled = true
			asserts.Equal(projectionStream, streamName)
			asserts.Equal(subscriptionGroup, groupName)

			return nil
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
				return fmt.Errorf("should return error")
			}

			return nil
		},
	}
	projectionConfig := account.NewProjectionConfig(eventHandler)
	eventMatcherHandlerRegistry, err := projector.NewEventMatcherHandlerRegistry(
		[]definitions.EventMatcherHandleProvider{projectionConfig},
	)
	asserts.NoError(err)

	var startCalled = false
	var stopCalled = false
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error { startCalled = true; return nil },
		OnStop:  func(context.Context) error { stopCalled = true; return nil },
	})

	err = account.SubscribeEventStream(
		client,
		eventMatcherHandlerRegistry,
		zaptest.NewLogger(t),
		lifecycle,
	)
	asserts.NoError(err)

	asserts.NoError(lifecycle.Start(ctx))
	asserts.True(startCalled, "lifecycle start was never called")

	keepRunning := true
	for {
		select {
		case <-subscriptionReceiveChannel:
			keepRunning = false
		case <-ctx.Done():
			keepRunning = false
		case <-time.After(1 * time.Second):
			keepRunning = false
		}

		if !keepRunning {
			break
		}
	}

	asserts.NoError(lifecycle.Stop(ctx))
	asserts.True(stopCalled, "lifecycle stop was never called")

	asserts.True(subscriptionCreateCalled, "CreatePersistentSubscription was never called")
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
	asserts.True(subscriptionEventAck, "Subscription::Ack was never called")
	asserts.True(subscriptionEventNack, "Subscription::Nack was never called")
}
